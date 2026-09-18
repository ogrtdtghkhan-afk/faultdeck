package deck

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode/utf8"
)

const Version = "0.2.0"

type Config struct {
	Version          string
	Upstream         string
	ProxyURL         string
	DemoURL          string
	AdminURL         string
	DataDir          string
	InternalProxyURL string
	InternalAdminURL string
	AdminUser        string
	AdminPassword    string
	ProxyToken       string
}

type Rule struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	DelayMS    int    `json:"delayMs"`
	StatusCode int    `json:"statusCode"`
	Every      int    `json:"every"`
	Limit      int    `json:"limit"`
	Matched    uint64 `json:"matched,omitempty"`
	Hits       uint64 `json:"hits,omitempty"`
}

type Scenario struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`
	Rules    []Rule `json:"rules"`
}

type Log struct {
	ID         uint64  `json:"id"`
	Time       string  `json:"time"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Status     int     `json:"status"`
	DurationMS float64 `json:"durationMs"`
	RuleID     string  `json:"ruleId,omitempty"`
	RuleName   string  `json:"ruleName,omitempty"`
	Fault      string  `json:"fault,omitempty"`
	Error      string  `json:"error,omitempty"`
}

type Stats struct {
	Requests      uint64  `json:"requests"`
	Injected      uint64  `json:"injected"`
	Errors        uint64  `json:"errors"`
	AvgDurationMS float64 `json:"avgDurationMs"`
}

type State struct {
	Persistent bool   `json:"persistent"`
	ProxyAuth  bool   `json:"proxyAuth"`
	Version    string `json:"version"`
	ProxyURL   string `json:"proxyUrl"`
	DemoURL    string `json:"demoUrl"`
	Upstream   string `json:"upstream"`
	Enabled    bool   `json:"enabled"`
	StartedAt  string `json:"startedAt"`
	Rules      []Rule `json:"rules"`
	Stats      Stats  `json:"stats"`
	Logs       []Log  `json:"logs"`
}

func validateRule(r *Rule) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || utf8.RuneCountInString(r.Name) > 80 {
		return fmt.Errorf("rule name must contain 1–80 characters")
	}
	r.Method = strings.ToUpper(r.Method)
	switch r.Method {
	case "*", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		return fmt.Errorf("unsupported request method")
	}
	if len(r.Path) > 1024 || (r.Path != "*" && !strings.HasPrefix(r.Path, "/")) || strings.ContainsAny(r.Path, "?#\r\n") || strings.Contains(strings.TrimSuffix(r.Path, "*"), "*") {
		return fmt.Errorf("path must be exact, *, or a prefix ending in *")
	}
	switch r.Type {
	case "latency", "status", "timeout", "disconnect":
	default:
		return fmt.Errorf("unknown fault type")
	}
	if r.DelayMS < 0 || r.DelayMS > 60000 {
		return fmt.Errorf("delay must be between 0 and 60000 ms")
	}
	if r.Type == "timeout" && r.DelayMS == 0 {
		return fmt.Errorf("timeout delay must be at least 1 ms")
	}
	if r.StatusCode == 0 {
		r.StatusCode = 503
	}
	if r.StatusCode < 400 || r.StatusCode > 599 {
		return fmt.Errorf("status must be between 400 and 599")
	}
	if r.Every == 0 {
		r.Every = 1
	}
	if r.Every < 1 || r.Every > 100000 || r.Limit < 0 || r.Limit > 100000 {
		return fmt.Errorf("every must be 1–100000; limit must be 0–100000")
	}
	r.Matched, r.Hits = 0, 0
	return nil
}

func (d *Deck) validateTarget(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return nil, fmt.Errorf("target must be an http(s) URL without credentials, query or fragment")
	}
	if p := u.Port(); p != "" {
		if _, err := net.LookupPort("tcp", p); err != nil {
			return nil, fmt.Errorf("invalid target port")
		}
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		for _, own := range []string{d.config.ProxyURL, d.config.AdminURL, d.config.InternalProxyURL, d.config.InternalAdminURL} {
			v, _ := url.Parse(own)
			if v != nil && effectivePort(u) == effectivePort(v) {
				return nil, fmt.Errorf("target cannot point to FaultDeck's proxy or control port")
			}
		}
	}
	return u, nil
}

func effectivePort(u *url.URL) string {
	if u.Port() != "" {
		return u.Port()
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

func pathMatches(pattern, path string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == path
}
