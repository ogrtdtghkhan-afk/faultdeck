package deck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInvalidScenarioImportIsAtomic(t *testing.T) {
	d := newProxyTestDeck(t, "http://127.0.0.1:8000")
	addProxyTestRule(t, d, Rule{Name: "original", Enabled: true, Method: "GET", Path: "*", Type: "status", StatusCode: 429})
	d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/orders", nil))
	before := d.State()
	beforeExport := d.Export()
	invalid := Scenario{Version: 1, Name: "replacement", Upstream: "http://127.0.0.1:9000", Enabled: false, Rules: []Rule{
		{Name: "valid first rule", Enabled: true, Method: "*", Path: "*", Type: "status", StatusCode: 503},
		{Name: "invalid second rule", Enabled: true, Method: "GET", Path: "/api/*/bad", Type: "status", StatusCode: 503},
	}}
	if err := d.Import(invalid); err == nil {
		t.Fatal("invalid scenario was accepted")
	}
	if after := d.State(); !reflect.DeepEqual(before, after) {
		t.Fatalf("failed import changed state\nbefore: %+v\nafter: %+v", before, after)
	}
	if after := d.Export(); !reflect.DeepEqual(beforeExport, after) {
		t.Fatalf("failed import changed exported scenario: %+v", after)
	}
}

func TestScenarioRoundTripResetsRuntimeAndAssignsUniqueIDs(t *testing.T) {
	d := newProxyTestDeck(t, "http://127.0.0.1:8000")
	original := addProxyTestRule(t, d, Rule{Name: "original", Enabled: true, Method: "GET", Path: "*", Type: "status", StatusCode: 503, Every: 1, Limit: 2})
	d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/orders", nil))
	scenario := d.Export()
	if scenario.Rules[0].Matched != 0 || scenario.Rules[0].Hits != 0 {
		t.Fatal("export retained runtime counters")
	}
	// Caller-supplied counters and duplicate identifiers must not survive import.
	scenario.Rules[0].Matched, scenario.Rules[0].Hits = 100, 100
	scenario.Rules = append(scenario.Rules, scenario.Rules[0])
	if err := d.Import(scenario); err != nil {
		t.Fatal(err)
	}
	state := d.State()
	if len(state.Logs) != 0 || state.Stats != (Stats{}) || state.Rules[0].Hits != 0 || state.Rules[0].Matched != 0 {
		t.Fatalf("import did not reset runtime state: %+v", state)
	}
	if state.Rules[0].ID == original.ID || state.Rules[0].ID == state.Rules[1].ID {
		t.Fatalf("imported IDs were not fresh and unique: %+v", state.Rules)
	}
}

func TestStateSnapshotsCannotMutateDeck(t *testing.T) {
	d := newProxyTestDeck(t, "http://127.0.0.1:8000")
	addProxyTestRule(t, d, Rule{Name: "original", Enabled: true, Method: "GET", Path: "*", Type: "status", StatusCode: 503})
	d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/orders", nil))
	state := d.State()
	state.Rules[0].Name = "mutated"
	state.Logs[0].Path = "/mutated"
	if fresh := d.State(); fresh.Rules[0].Name != "original" || fresh.Logs[0].Path != "/orders" {
		t.Fatal("State returned writable shared slices")
	}
}

func TestLogsBoundedAndNewestFirst(t *testing.T) {
	d := newProxyTestDeck(t, "http://127.0.0.1:8000")
	addProxyTestRule(t, d, Rule{Name: "error", Enabled: true, Method: "GET", Path: "*", Type: "status", StatusCode: 503})
	for range 205 {
		d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/orders", nil))
	}
	state := d.State()
	if len(state.Logs) != 200 || state.Stats.Requests != 205 || state.Stats.Injected != 205 {
		t.Fatalf("log cap or lifetime totals incorrect: logs=%d stats=%+v", len(state.Logs), state.Stats)
	}
	for i := 1; i < len(state.Logs); i++ {
		if state.Logs[i-1].ID <= state.Logs[i].ID {
			t.Fatalf("logs not newest first at index %d", i)
		}
	}
}

func TestResetExcludesRequestsAlreadyInFlight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		w.WriteHeader(204)
	}))
	t.Cleanup(upstream.Close)
	d := newProxyTestDeck(t, upstream.URL)
	done := make(chan struct{})
	go func() {
		d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/first", nil))
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("upstream request did not start")
	}
	d.Reset()
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request did not finish")
	}
	if state := d.State(); len(state.Logs) != 0 || state.Stats.Requests != 0 {
		t.Fatalf("pre-reset request reappeared: %+v", state)
	}
	d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/second", nil))
	if state := d.State(); len(state.Logs) != 1 || state.Logs[0].Path != "/second" || state.Stats.Requests != 1 {
		t.Fatalf("post-reset request missing: %+v", state)
	}
}

func TestConcurrentRequestsAndReset(t *testing.T) {
	d := newProxyTestDeck(t, "http://127.0.0.1:8000")
	addProxyTestRule(t, d, Rule{Name: "error", Enabled: true, Method: "GET", Path: "*", Type: "status", StatusCode: 503})
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range 100 {
				if worker == 0 && i%10 == 0 {
					d.Reset()
				}
				recorder := httptest.NewRecorder()
				d.ProxyHandler().ServeHTTP(recorder, httptest.NewRequest("GET", "/orders", nil))
				if recorder.Code != 503 {
					t.Errorf("concurrent fault request returned %d", recorder.Code)
				}
				_ = d.State()
				_ = d.Export()
			}
		}()
	}
	workers.Wait()
	state := d.State()
	if state.Stats.Requests != state.Stats.Injected || state.Stats.Requests != state.Stats.Errors || state.Rules[0].Matched != state.Rules[0].Hits || len(state.Logs) > 200 {
		t.Fatalf("concurrent counters inconsistent: %+v", state)
	}
	d.Reset()
	d.ProxyHandler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/fresh", nil))
	state = d.State()
	if state.Stats.Requests != 1 || state.Rules[0].Matched != 1 || state.Rules[0].Hits != 1 || len(state.Logs) != 1 {
		t.Fatalf("clean run after concurrency incorrect: %+v", state)
	}
}

func TestBundledScenariosImport(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("find scenario fixtures: %v (files=%d)", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			var scenario Scenario
			decoder := json.NewDecoder(file)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&scenario); err != nil {
				t.Fatal(err)
			}
			d := newProxyTestDeck(t, "http://127.0.0.1:7333")
			if err := d.Import(scenario); err != nil {
				t.Fatal(err)
			}
			if len(d.State().Rules) == 0 {
				t.Fatal("scenario did not create any rules")
			}
		})
	}
}
