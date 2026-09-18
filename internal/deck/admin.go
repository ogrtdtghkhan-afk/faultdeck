package deck

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON request: %v", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("request must contain exactly one JSON value")
	}
	return nil
}

func apiError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (d *Deck) AdminHandler(ui http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, d.State()) })
	mux.HandleFunc("PUT /api/config", d.configure)
	mux.HandleFunc("POST /api/rules", func(w http.ResponseWriter, r *http.Request) {
		var rule Rule
		if err := decodeJSON(w, r, &rule); err != nil {
			apiError(w, 400, err)
			return
		}
		created, err := d.AddRule(rule)
		if err != nil {
			apiError(w, 400, err)
			return
		}
		writeJSON(w, 201, created)
	})
	mux.HandleFunc("PUT /api/rules/{id}", d.updateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		for i := range d.rules {
			if d.rules[i].ID == r.PathValue("id") {
				d.rules = append(d.rules[:i], d.rules[i+1:]...)
				w.WriteHeader(204)
				return
			}
		}
		apiError(w, 404, fmt.Errorf("rule not found"))
	})
	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) { d.Reset(); writeJSON(w, 200, d.State()) })
	mux.HandleFunc("DELETE /api/logs", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		d.logs = []Log{}
		d.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /api/scenario", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="faultdeck-scenario.json"`)
		writeJSON(w, 200, d.Export())
	})
	mux.HandleFunc("POST /api/scenario", func(w http.ResponseWriter, r *http.Request) {
		var scenario Scenario
		if err := decodeJSON(w, r, &scenario); err != nil {
			apiError(w, 400, err)
			return
		}
		if err := d.Import(scenario); err != nil {
			apiError(w, 400, err)
			return
		}
		writeJSON(w, 200, d.State())
	})
	mux.HandleFunc("POST /api/play", d.play)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { apiError(w, 404, fmt.Errorf("API endpoint not found")) })
	mux.Handle("/", ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		hostname := r.Host
		if host, _, err := net.SplitHostPort(r.Host); err == nil {
			hostname = host
		}
		hostname = strings.TrimSuffix(strings.ToLower(hostname), ".")
		ip := net.ParseIP(hostname)
		if hostname != "localhost" && (ip == nil || !ip.IsLoopback()) {
			apiError(w, 403, fmt.Errorf("control interface requires a loopback hostname"))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			apiError(w, 403, fmt.Errorf("cross-origin access denied"))
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			apiError(w, 403, fmt.Errorf("cross-site access denied"))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-FaultDeck") != "1" {
			apiError(w, 403, fmt.Errorf("missing X-FaultDeck: 1 header"))
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (d *Deck) configure(w http.ResponseWriter, r *http.Request) {
	var change struct {
		Upstream *string `json:"upstream"`
		Enabled  *bool   `json:"enabled"`
	}
	if err := decodeJSON(w, r, &change); err != nil {
		apiError(w, 400, err)
		return
	}
	var target *url.URL
	if change.Upstream != nil {
		var err error
		target, err = d.validateTarget(*change.Upstream)
		if err != nil {
			apiError(w, 400, err)
			return
		}
	}
	d.mu.Lock()
	if target != nil {
		d.target = target
	}
	if change.Enabled != nil {
		d.enabled = *change.Enabled
	}
	state := d.stateLocked()
	d.mu.Unlock()
	writeJSON(w, 200, state)
}

func (d *Deck) updateRule(w http.ResponseWriter, r *http.Request) {
	var rule Rule
	if err := decodeJSON(w, r, &rule); err != nil {
		apiError(w, 400, err)
		return
	}
	if err := validateRule(&rule); err != nil {
		apiError(w, 400, err)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.rules {
		if d.rules[i].ID == r.PathValue("id") {
			rule.ID = d.rules[i].ID
			d.rules[i] = rule
			writeJSON(w, 200, rule)
			return
		}
	}
	apiError(w, 404, fmt.Errorf("rule not found"))
}

func (d *Deck) play(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, 400, err)
		return
	}
	if input.Method != "GET" && input.Method != "HEAD" {
		apiError(w, 400, fmt.Errorf("playground accepts GET and HEAD only"))
		return
	}
	u, err := url.ParseRequestURI(input.Path)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(input.Path, "/") || strings.HasPrefix(input.Path, "//") || strings.Contains(input.Path, "#") {
		apiError(w, 400, fmt.Errorf("enter a relative path beginning with /"))
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), input.Method, d.config.ProxyURL+input.Path, nil)
	if err != nil {
		apiError(w, 400, fmt.Errorf("invalid request path"))
		return
	}
	// A dedicated transport avoids implicit retries on reused GET connections, which
	// would hide a deliberately injected disconnect in this one-request playground.
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	start := time.Now()
	response, requestErr := client.Do(request)
	result := struct {
		Status     int     `json:"status"`
		DurationMS float64 `json:"durationMs"`
		Body       string  `json:"body"`
		Error      string  `json:"error"`
	}{}
	if requestErr != nil {
		result.Error = "Connection closed or request failed. See activity for the injected fault."
		if e, ok := requestErr.(net.Error); ok && e.Timeout() {
			result.Error = "Playground deadline exceeded (10 seconds)."
		}
	} else {
		defer response.Body.Close()
		result.Status = response.StatusCode
		body, readErr := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
		if len(body) > 64<<10 {
			body = append(body[:64<<10], []byte("\n[response truncated at 64 KiB]")...)
		}
		result.Body = string(body)
		if readErr != nil {
			result.Error = "Response interrupted before completion."
		}
	}
	result.DurationMS = float64(time.Since(start).Microseconds()) / 1000
	writeJSON(w, 200, result)
}
