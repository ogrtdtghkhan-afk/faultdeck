package deck

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticatedControlOrigin(t *testing.T) {
	d := adminTestDeck(t)
	d.config.AdminURL = "https://faultdeck.example.test"
	d.config.AdminUser, d.config.AdminPassword = "admin", "test-admin-secret"
	cases := []struct {
		name, host, origin, user, password string
		status                             int
	}{
		{"login required", "faultdeck.example.test", "", "", "", 401},
		{"wrong password", "faultdeck.example.test", "", "admin", "wrong-secret", 401},
		{"wrong user", "faultdeck.example.test", "", "other", "test-admin-secret", 401},
		{"public HTTPS", "faultdeck.example.test", "https://faultdeck.example.test", "admin", "test-admin-secret", 200},
		{"wrong scheme", "faultdeck.example.test", "http://faultdeck.example.test", "admin", "test-admin-secret", 403},
		{"untrusted host", "evil.example.test", "", "admin", "test-admin-secret", 403},
		{"cross origin with credentials", "faultdeck.example.test", "https://evil.example.test", "admin", "test-admin-secret", 403},
		{"local tunnel", "localhost:7331", "http://localhost:7331", "admin", "test-admin-secret", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+tc.host+"/api/state", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Forwarded-Host", "faultdeck.example.test")
			if tc.user != "" {
				r.SetBasicAuth(tc.user, tc.password)
			}
			w := httptest.NewRecorder()
			d.AdminHandler(http.NotFoundHandler()).ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if w.Code == 401 && !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Basic ") {
				t.Fatal("browser login challenge missing")
			}
			if strings.Contains(w.Body.String(), d.config.AdminPassword) {
				t.Fatal("password leaked in response")
			}
		})
	}
	for _, path := range []string{"/", "/app.js", "/api/scenario"} {
		r := httptest.NewRequest("GET", "https://faultdeck.example.test"+path, nil)
		w := httptest.NewRecorder()
		d.AdminHandler(http.NotFoundHandler()).ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("unprotected path %s: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "http://internal-container:7331/healthz", nil)
	w := httptest.NewRecorder()
	d.AdminHandler(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("healthcheck must work without disclosing state")
	}
}

func TestProxyTokenAndBusinessCredentials(t *testing.T) {
	var called int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.RequestURI() != "/orders?limit=2" || string(body) != `{"item":"book"}` {
			t.Errorf("request changed: %s %s %s", r.Method, r.URL, body)
		}
		if r.Header.Get("Authorization") != "Bearer business-secret" {
			t.Error("business authorization lost")
		}
		if r.Header.Get("X-FaultDeck-Token") != "" {
			t.Error("deployment token forwarded upstream")
		}
		w.WriteHeader(201)
	}))
	defer upstream.Close()
	d := adminTestDeck(t)
	d.config.ProxyToken = "test-proxy-secret"
	if _, err := d.Configure(&upstream.URL, nil); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", "incorrect-token", "test-proxy-secret"} {
		r := httptest.NewRequest("POST", "http://localhost:7332/orders?limit=2", strings.NewReader(`{"item":"book"}`))
		r.Header.Set("Authorization", "Bearer business-secret")
		r.Header.Set("X-FaultDeck-Token", token)
		w := httptest.NewRecorder()
		d.ProxyHandler().ServeHTTP(w, r)
		expected := 401
		if token == d.config.ProxyToken {
			expected = 201
		}
		if w.Code != expected {
			t.Fatalf("got %d, want %d", w.Code, expected)
		}
	}
	if called != 1 || d.State().Stats.Requests != 1 {
		t.Fatal("unauthenticated traffic reached proxy/rule counters")
	}
}

func TestPlaygroundUsesInternalAuthenticatedProxy(t *testing.T) {
	upstream := httptest.NewServer(DemoHandler())
	defer upstream.Close()
	d := adminTestDeck(t)
	d.config.ProxyToken = "test-proxy-secret"
	if _, err := d.Configure(&upstream.URL, nil); err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(d.ProxyHandler())
	defer proxy.Close()
	d.config.ProxyURL, d.config.InternalProxyURL = "https://unreachable.example.test", proxy.URL
	w := adminRequest(d, "POST", "/api/play", `{"method":"GET","path":"/api/orders"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":200`) || !strings.Contains(w.Body.String(), "ORD-1042") {
		t.Fatal(w.Code, w.Body.String())
	}
	if !d.State().ProxyAuth {
		t.Fatal("missing proxy authentication indicator")
	}
}
