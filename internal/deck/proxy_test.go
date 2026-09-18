package deck

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newProxyTestDeck(t *testing.T, upstream string) *Deck {
	t.Helper()
	d, err := New(Config{Upstream: upstream})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

func addProxyTestRule(t *testing.T, d *Deck, rule Rule) Rule {
	t.Helper()
	added, err := d.AddRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	return added
}

func waitProxyTest(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true before deadline")
}

func TestProxyPreservesRequestAndResponse(t *testing.T) {
	type capturedRequest struct {
		method, path, query, body, header, host string
	}
	seen := make(chan capturedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen <- capturedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(body), r.Header.Get("Authorization"), r.Host}
		w.Header().Set("X-Upstream-Test", "forwarded")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "private-response-body")
	}))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL+"/base")
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/orders?token=private-query&encoded=a%2Fb", strings.NewReader("private-request-body"))
	req.Header.Set("Authorization", "Bearer private-header")
	recorder := httptest.NewRecorder()
	d.ProxyHandler().ServeHTTP(recorder, req)
	got := <-seen
	if got.method != "POST" || got.path != "/base/orders" || got.query != "token=private-query&encoded=a%2Fb" || got.body != "private-request-body" || got.header != "Bearer private-header" || got.host != strings.TrimPrefix(upstream.URL, "http://") {
		t.Fatalf("request was changed unexpectedly: %+v", got)
	}
	if recorder.Code != http.StatusAccepted || recorder.Body.String() != "private-response-body" || recorder.Header().Get("X-Upstream-Test") != "forwarded" {
		t.Fatalf("response was changed: %d, %q, %v", recorder.Code, recorder.Body.String(), recorder.Header())
	}
	state := d.State()
	if len(state.Logs) != 1 || state.Logs[0].Path != "/orders" || state.Logs[0].Method != "POST" || state.Logs[0].Status != 202 {
		t.Fatalf("incorrect activity metadata: %+v", state.Logs)
	}
	encoded, err := json.Marshal(state.Logs)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-query", "private-header", "private-request-body", "private-response-body"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("activity log leaked %q", secret)
		}
	}
}

func TestRuleOrderEveryAndLimit(t *testing.T) {
	var forwarded atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	primary := addProxyTestRule(t, d, Rule{Name: "every second, twice", Enabled: true, Method: "GET", Path: "/api/orders", Type: "status", StatusCode: 503, Every: 2, Limit: 2})
	fallback := addProxyTestRule(t, d, Rule{Name: "one fallback", Enabled: true, Method: "GET", Path: "/api/*", Type: "status", StatusCode: 418, Every: 1, Limit: 1})
	handler := d.ProxyHandler()
	for i, expected := range []int{418, 503, 200, 503, 200} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", "/api/orders", nil))
		if r.Code != expected {
			t.Errorf("request %d: got %d, want %d", i+1, r.Code, expected)
		}
	}
	for _, request := range []struct{ method, path string }{{"POST", "/api/orders"}, {"GET", "/outside"}} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest(request.method, request.path, nil))
		if r.Code != 200 {
			t.Errorf("nonmatching request %s %s got %d", request.method, request.path, r.Code)
		}
	}
	state := d.State()
	if state.Rules[0].ID != primary.ID || state.Rules[0].Matched != 5 || state.Rules[0].Hits != 2 {
		t.Errorf("primary counters: %+v", state.Rules[0])
	}
	if state.Rules[1].ID != fallback.ID || state.Rules[1].Matched != 3 || state.Rules[1].Hits != 1 {
		t.Errorf("fallback counters: %+v", state.Rules[1])
	}
	if forwarded.Load() != 4 || state.Stats.Requests != 7 || state.Stats.Injected != 3 || state.Stats.Errors != 3 {
		t.Errorf("forwarded=%d stats=%+v", forwarded.Load(), state.Stats)
	}
}

func TestGlobalSwitchBypassesRulesWithoutAdvancingCounters(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	addProxyTestRule(t, d, Rule{Name: "disabled globally", Enabled: true, Method: "*", Path: "*", Type: "status", StatusCode: 503})
	scenario := d.Export()
	scenario.Enabled = false
	if err := d.Import(scenario); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	d.ProxyHandler().ServeHTTP(r, httptest.NewRequest("GET", "/api/orders", nil))
	state := d.State()
	if r.Code != 204 || state.Stats.Injected != 0 || state.Rules[0].Matched != 0 || state.Rules[0].Hits != 0 {
		t.Fatalf("global off did not bypass rules: status=%d state=%+v", r.Code, state)
	}
}

func TestLatencyForwardsAfterDelay(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) }))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	addProxyTestRule(t, d, Rule{Name: "slow", Enabled: true, Method: "GET", Path: "*", Type: "latency", DelayMS: 25})
	r := httptest.NewRecorder()
	start := time.Now()
	d.ProxyHandler().ServeHTTP(r, httptest.NewRequest("GET", "/", nil))
	if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
		t.Errorf("latency returned too early: %s", elapsed)
	}
	if r.Code != 201 || d.State().Logs[0].Fault != "latency" {
		t.Fatalf("latency did not forward upstream response: %d %+v", r.Code, d.State().Logs)
	}
}

func TestCancelDuringLatencyDoesNotReachUpstream(t *testing.T) {
	var forwarded atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	addProxyTestRule(t, d, Rule{Name: "long wait", Enabled: true, Method: "GET", Path: "*", Type: "latency", DelayMS: 60000})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	waitProxyTest(t, func() bool { return d.State().Rules[0].Hits == 1 })
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not interrupt injected delay")
	}
	state := d.State()
	if forwarded.Load() != 0 || len(state.Logs) != 1 || state.Logs[0].Status != 0 || state.Logs[0].Error == "" {
		t.Fatalf("cancellation reached upstream or was not recorded: forwarded=%d logs=%+v", forwarded.Load(), state.Logs)
	}
}

func TestTimeoutReturns504WithoutForwarding(t *testing.T) {
	var forwarded atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	addProxyTestRule(t, d, Rule{Name: "timeout", Enabled: true, Method: "GET", Path: "*", Type: "timeout", DelayMS: 25})
	r := httptest.NewRecorder()
	start := time.Now()
	d.ProxyHandler().ServeHTTP(r, httptest.NewRequest("GET", "/", nil))
	if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
		t.Errorf("timeout returned too early: %s", elapsed)
	}
	if r.Code != 504 || forwarded.Load() != 0 || r.Header().Get("X-FaultDeck-Fault") != "timeout" {
		t.Fatalf("timeout response=%d forwarded=%d", r.Code, forwarded.Load())
	}
}

func TestDisconnectClosesTCPWithoutResponse(t *testing.T) {
	var forwarded atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	addProxyTestRule(t, d, Rule{Name: "drop", Enabled: true, Method: "GET", Path: "*", Type: "disconnect"})
	proxy := httptest.NewServer(d.ProxyHandler())
	t.Cleanup(proxy.Close)
	conn, err := net.DialTimeout("tcp", proxy.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET /api/orders HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	_, err = bufio.NewReader(conn).ReadByte()
	if err != io.EOF {
		t.Fatalf("wanted connection EOF without an HTTP response, got %v", err)
	}
	waitProxyTest(t, func() bool { return d.State().Stats.Requests == 1 })
	state := d.State()
	if forwarded.Load() != 0 || state.Logs[0].Status != 0 || state.Logs[0].Fault != "disconnect" || state.Logs[0].Error == "" {
		t.Fatalf("disconnect was forwarded or logged incorrectly: forwarded=%d logs=%+v", forwarded.Load(), state.Logs)
	}
}

func TestProxyLoopGuard(t *testing.T) {
	d := newProxyTestDeck(t, "http://127.0.0.1:1")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-FaultDeck-Hop", "1")
	r := httptest.NewRecorder()
	d.ProxyHandler().ServeHTTP(r, req)
	if r.Code != http.StatusLoopDetected || d.State().Logs[0].Error != "proxy loop detected" {
		t.Fatalf("loop guard failed: status=%d logs=%+v", r.Code, d.State().Logs)
	}
}
