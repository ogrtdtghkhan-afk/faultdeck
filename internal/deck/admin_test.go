package deck

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func adminTestDeck(t *testing.T) *Deck {
	t.Helper()
	d, err := New(Config{Upstream: "http://127.0.0.1:7333", ProxyURL: "http://127.0.0.1:7332", AdminURL: "http://127.0.0.1:7331", DemoURL: "http://127.0.0.1:7333"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

func adminRequest(d *Deck, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:7331"+path, strings.NewReader(body))
	r.Header.Set("X-FaultDeck", "1")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.AdminHandler(http.NotFoundHandler()).ServeHTTP(w, r)
	return w
}

func TestAdminOriginAndHostProtection(t *testing.T) {
	d := adminTestDeck(t)
	cases := []struct {
		name, host, origin, site, header string
		status                           int
	}{
		{"normal", "127.0.0.1:7331", "http://127.0.0.1:7331", "same-origin", "1", 200},
		{"cross origin", "127.0.0.1:7331", "https://evil.example", "", "1", 403},
		{"null origin", "127.0.0.1:7331", "null", "", "1", 403},
		{"rebinding", "evil.example:7331", "", "", "1", 403},
		{"missing header", "127.0.0.1:7331", "", "", "", 403},
		{"cross site", "127.0.0.1:7331", "", "cross-site", "1", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("PUT", "http://127.0.0.1:7331/api/config", strings.NewReader(`{"enabled":false}`))
			r.Host = tc.host
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			r.Header.Set("X-FaultDeck", tc.header)
			w := httptest.NewRecorder()
			d.AdminHandler(http.NotFoundHandler()).ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestAdminValidationIsAtomic(t *testing.T) {
	d := adminTestDeck(t)
	for _, target := range []string{"ftp://example.com", "http://user:password@example.com", "http://example.com?token=secret", "http://localhost:7332", "http://127.0.0.1:7331", "http://[::1]:7332", "http://localhost:99999"} {
		body, _ := json.Marshal(map[string]any{"upstream": target, "enabled": false})
		w := adminRequest(d, "PUT", "/api/config", string(body))
		if w.Code != 400 {
			t.Fatalf("target %s accepted: %d", target, w.Code)
		}
		if !d.State().Enabled || d.State().Upstream != "http://127.0.0.1:7333" {
			t.Fatal("invalid configuration partially applied")
		}
	}
	for _, body := range []string{`{"enabled":false,"typo":1}`, `{"enabled":false} {"enabled":true}`, `null null`, `{"enabled":`} {
		if w := adminRequest(d, "PUT", "/api/config", body); w.Code != 400 {
			t.Fatalf("invalid body accepted: %q, %d", body, w.Code)
		}
	}
	if !d.State().Enabled {
		t.Fatal("malformed JSON changed config")
	}
}

func TestAdminRuleLifecycleAndScenario(t *testing.T) {
	d := adminTestDeck(t)
	w := adminRequest(d, "POST", "/api/rules", `{"name":"Fail twice","enabled":true,"method":"GET","path":"/api/orders","type":"status","statusCode":503,"every":1,"limit":2}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created Rule
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("missing generated ID")
	}
	created.Enabled = false
	body, _ := json.Marshal(created)
	w = adminRequest(d, "PUT", "/api/rules/"+created.ID, string(body))
	if w.Code != 200 || d.State().Rules[0].Enabled {
		t.Fatal("rule update failed")
	}
	exported := adminRequest(d, "GET", "/api/scenario", "")
	if exported.Code != 200 {
		t.Fatal(exported.Body.String())
	}
	if strings.Contains(exported.Body.String(), `"matched"`) || strings.Contains(exported.Body.String(), `"hits"`) {
		t.Fatal("export contains runtime counters")
	}
	if w := adminRequest(d, "DELETE", "/api/rules/"+created.ID, ""); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if len(d.State().Rules) != 0 {
		t.Fatal("rule not removed")
	}
	if w := adminRequest(d, "POST", "/api/scenario", exported.Body.String()); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(d.State().Rules) != 1 || d.State().Rules[0].ID == created.ID {
		t.Fatal("import must allocate a fresh ID")
	}
	if w := adminRequest(d, "DELETE", "/api/rules/missing", ""); w.Code != 404 {
		t.Fatal("missing rule should return 404")
	}
}

func TestAdminRejectsOversizeImport(t *testing.T) {
	d := adminTestDeck(t)
	data := `{"name":"` + strings.Repeat("x", (1<<20)+20) + `"}`
	if w := adminRequest(d, "POST", "/api/scenario", data); w.Code != 400 {
		t.Fatal("oversize request accepted", w.Code)
	}
}

func TestPlaygroundTraversesProxy(t *testing.T) {
	upstream := httptest.NewServer(DemoHandler())
	defer upstream.Close()
	d := adminTestDeck(t)
	u, err := d.validateTarget(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	d.target = u
	proxy := httptest.NewServer(d.ProxyHandler())
	defer proxy.Close()
	d.config.ProxyURL = proxy.URL
	_, err = d.AddRule(Rule{Name: "One error", Enabled: true, Method: "GET", Path: "/api/orders", Type: "status", StatusCode: 429, Every: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	for i, expected := range []int{429, 200} {
		w := adminRequest(d, "POST", "/api/play", `{"method":"GET","path":"/api/orders"}`)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct {
			Status int    `json:"status"`
			Body   string `json:"body"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != expected {
			t.Fatalf("request %d status %d, want %d", i, result.Status, expected)
		}
		if i == 1 && !strings.Contains(result.Body, "ORD-1042") {
			t.Fatal("missing real upstream response")
		}
	}
	if d.State().Stats.Requests != 2 || d.State().Stats.Injected != 1 {
		t.Fatal("playground bypassed proxy counters", d.State().Stats)
	}
	for _, body := range []string{`{"method":"POST","path":"/api/orders"}`, `{"method":"GET","path":"http://example.com"}`, `{"method":"GET","path":"//example.com"}`} {
		if w := adminRequest(d, "POST", "/api/play", body); w.Code != 400 {
			t.Fatal("unsafe playground input accepted", body)
		}
	}
}

func TestControlAPIThroughRealServer(t *testing.T) {
	d := adminTestDeck(t)
	server := httptest.NewServer(d.AdminHandler(http.NotFoundHandler()))
	defer server.Close()
	request, _ := http.NewRequest("PUT", server.URL+"/api/config", bytes.NewBufferString(`{"enabled":false}`))
	request.Header.Set("X-FaultDeck", "1")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 || d.State().Enabled {
		t.Fatal(response.StatusCode, string(body))
	}
}
