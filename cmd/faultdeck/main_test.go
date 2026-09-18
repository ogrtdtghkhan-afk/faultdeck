package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeploymentConfiguration(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		env     map[string]string
		wantErr bool
	}{
		{"local defaults", nil, nil, false},
		{"remote refuses anonymous", []string{"--listen", "0.0.0.0"}, nil, true},
		{"remote password", []string{"--listen", "0.0.0.0"}, map[string]string{"FAULTDECK_ADMIN_PASSWORD": "long-test-password"}, false},
		{"external origin needs password", []string{"--ui-origin", "https://faultdeck.example.test"}, nil, true},
		{"weak password", nil, map[string]string{"FAULTDECK_ADMIN_PASSWORD": "short"}, true},
		{"weak token", nil, map[string]string{"FAULTDECK_PROXY_TOKEN": "short"}, true},
		{"header injection", nil, map[string]string{"FAULTDECK_PROXY_TOKEN": "long-token\r\nInjected: yes"}, true},
		{"invalid username", nil, map[string]string{"FAULTDECK_ADMIN_USER": "name:password"}, true},
		{"same ports", []string{"--port", "7331"}, nil, true},
		{"invalid port", []string{"--port", "65536"}, nil, true},
		{"hostname binding", []string{"--listen", "example.test"}, nil, true},
		{"origin has path", []string{"--ui-origin", "https://example.test/admin"}, nil, true},
		{"origin has credentials", []string{"--proxy-url", "http://user:secret@example.test"}, nil, true},
		{"origin has query", []string{"--proxy-url", "http://example.test?secret"}, nil, true},
		{"origin bad port", []string{"--proxy-url", "http://example.test:99999"}, nil, true},
		{"conflicting startup options", []string{"--target", "http://localhost:8080", "--scenario", "x.json"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := parseOptions(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			c, err := o.config(func(key string) string { return tc.env[key] })
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
			if err == nil {
				if c.AdminPassword != "" && tc.env["FAULTDECK_PROXY_TOKEN"] == "" && c.ProxyToken != c.AdminPassword {
					t.Fatal("default proxy token differs from password")
				}
				if c.AdminUser != "admin" || c.DataDir != "./data" {
					t.Fatal("deployment defaults changed")
				}
			}
		})
	}
}

func TestPublicURLsDoNotChangeInternalRouting(t *testing.T) {
	o, err := parseOptions([]string{"--listen", "0.0.0.0", "--ui-origin", "https://faultdeck.example.test/", "--proxy-url", "https://proxy.example.test/"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := o.config(func(key string) string {
		if key == "FAULTDECK_ADMIN_PASSWORD" {
			return "long-test-password"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminURL != "https://faultdeck.example.test" || c.ProxyURL != "https://proxy.example.test" || c.InternalProxyURL != "http://127.0.0.1:7332" || c.InternalAdminURL != "http://127.0.0.1:7331" {
		t.Fatal("public URLs changed internal listeners")
	}
	if c.Upstream != "http://127.0.0.1:7333" {
		t.Fatal("default demo target changed")
	}
	for _, ip := range []string{"::", "::1", "192.0.2.1"} {
		o.listen = ip
		c, err := o.config(func(key string) string {
			if key == "FAULTDECK_ADMIN_PASSWORD" {
				return "long-test-password"
			}
			return ""
		})
		if err != nil || strings.Contains(c.InternalAdminURL, "0.0.0.0") {
			t.Fatal("invalid internal routing", err)
		}
	}
}

func TestHealthCheckRejectsRedirectAndFailure(t *testing.T) {
	status := 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Error("unexpected health path")
		}
		w.Header().Set("Location", "http://example.test")
		w.WriteHeader(status)
	}))
	defer server.Close()
	if err := checkHealth(server.URL); err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{301, 401, 503} {
		status = code
		if err := checkHealth(server.URL); err == nil {
			t.Fatalf("unhealthy status %d accepted", code)
		}
	}
}

func TestScenarioFileRejectsMalformedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario.json")
	for _, content := range []string{`{} {}`, `{"unexpected":true}`, strings.Repeat(" ", (1<<20)+1)} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadScenario(path); err == nil {
			t.Fatal("invalid scenario accepted")
		}
	}
}
