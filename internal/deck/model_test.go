package deck

import "testing"

func TestAdvertisedPortsDoNotReplaceInternalListeners(t *testing.T) {
	cases := []struct {
		name, upstream, proxyURL, adminURL string
	}{
		{"mapped proxy uses demo port", "http://127.0.0.1:7333", "http://127.0.0.1:7333", "http://127.0.0.1:7331"},
		{"mapped admin uses demo port", "http://127.0.0.1:7333", "http://127.0.0.1:7332", "http://127.0.0.1:7333"},
		{"HTTPS advertised origins preserve demo", "http://127.0.0.1:7333", "https://faults.example.test", "https://faultdeck.example.test"},
		{"HTTPS advertised origins do not claim local port 443", "https://127.0.0.1:443", "https://faults.example.test", "https://faultdeck.example.test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := New(Config{
				Upstream:         tc.upstream,
				DemoURL:          "http://127.0.0.1:7333",
				ProxyURL:         tc.proxyURL,
				AdminURL:         tc.adminURL,
				InternalProxyURL: "http://127.0.0.1:7332",
				InternalAdminURL: "http://127.0.0.1:7331",
				DataDir:          t.TempDir(),
			})
			if err != nil {
				t.Fatalf("first startup rejected a distinct internal upstream: %v", err)
			}
			t.Cleanup(d.Close)
			if got := d.State().Upstream; got != tc.upstream {
				t.Fatalf("upstream = %q, want %q", got, tc.upstream)
			}
			for _, own := range []string{"http://127.0.0.1:7331", "http://127.0.0.1:7332", "http://localhost:7331", "http://[::1]:7332"} {
				if _, err := d.Configure(&own, nil); err == nil {
					t.Fatalf("accepted actual internal listener as upstream: %s", own)
				}
			}
		})
	}
}

func TestTargetValidationFallsBackPerMissingInternalURL(t *testing.T) {
	for _, mode := range []string{"both missing", "proxy missing", "admin missing"} {
		t.Run(mode, func(t *testing.T) {
			config := Config{
				Upstream:         "http://127.0.0.1:7333",
				ProxyURL:         "http://127.0.0.1:7332",
				AdminURL:         "http://127.0.0.1:7331",
				InternalProxyURL: "http://127.0.0.1:7332",
				InternalAdminURL: "http://127.0.0.1:7331",
			}
			if mode != "admin missing" {
				config.InternalProxyURL = ""
			} else {
				config.ProxyURL = "http://127.0.0.1:7333"
			}
			if mode != "proxy missing" {
				config.InternalAdminURL = ""
			} else {
				config.AdminURL = "http://127.0.0.1:7333"
			}
			d, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(d.Close)
			for _, own := range []string{"http://127.0.0.1:7331", "http://127.0.0.1:7332"} {
				if _, err := d.Configure(&own, nil); err == nil {
					t.Fatalf("missing internal URL disabled loop protection for %s", own)
				}
			}
		})
	}
}
