package deck

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

type observedWriter struct {
	http.ResponseWriter
	status int
}

func (w *observedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *observedWriter) WriteHeader(status int) {
	if status >= 200 && w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *observedWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
func (w *observedWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func waitFor(ctx context.Context, ms int) bool {
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (d *Deck) ProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.config.ProxyToken != "" && !secretEqual(r.Header.Get("X-FaultDeck-Token"), d.config.ProxyToken) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "valid X-FaultDeck-Token header required"})
			return
		}
		start := time.Now()
		target, rule, epoch := d.selectRule(r.Method, r.URL.Path)
		entry := Log{Time: start.UTC().Format(time.RFC3339Nano), Method: r.Method, Path: r.URL.Path}
		if len(entry.Path) > 2048 {
			entry.Path = entry.Path[:2048] + "…"
		}
		out := &observedWriter{ResponseWriter: w}
		defer func() {
			panicValue := recover()
			entry.Status, entry.DurationMS = out.status, float64(time.Since(start).Microseconds())/1000
			if panicValue != nil {
				entry.Error = "response stream interrupted"
			}
			d.record(entry, epoch)
			if panicValue != nil {
				panic(panicValue)
			}
		}()
		if r.Header.Get("X-FaultDeck-Hop") != "" {
			entry.Error = "proxy loop detected"
			writeJSON(out, http.StatusLoopDetected, map[string]string{"error": entry.Error})
			return
		}
		if rule != nil {
			entry.RuleID, entry.RuleName, entry.Fault = rule.ID, rule.Name, rule.Type
			switch rule.Type {
			case "status":
				if rule.StatusCode == 429 {
					out.Header().Set("Retry-After", "1")
				}
				out.Header().Set("X-FaultDeck-Fault", "status")
				writeJSON(out, rule.StatusCode, map[string]any{"error": "Injected by FaultDeck", "rule": rule.Name, "status": rule.StatusCode})
				return
			case "latency", "timeout":
				if !waitFor(r.Context(), rule.DelayMS) {
					entry.Error = "client canceled during injected delay"
					return
				}
				if rule.Type == "timeout" {
					out.Header().Set("X-FaultDeck-Fault", "timeout")
					writeJSON(out, http.StatusGatewayTimeout, map[string]string{"error": "Injected timeout by FaultDeck"})
					return
				}
			case "disconnect":
				conn, _, err := http.NewResponseController(w).Hijack()
				if err != nil {
					entry.Error = "connection hijacking unavailable"
					writeJSON(out, 501, map[string]string{"error": entry.Error})
					return
				}
				_ = conn.Close()
				entry.Error = "connection closed by FaultDeck"
				return
			}
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.Out.Host = target.Host
				pr.Out.Header.Del("X-FaultDeck-Token")
				pr.Out.Header.Set("X-FaultDeck-Hop", "1")
			},
			Transport:     d.transport,
			FlushInterval: -1,
			ErrorLog:      log.New(io.Discard, "", 0),
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				entry.Error = "upstream request failed"
				if r.Context().Err() != nil {
					entry.Error = "client canceled request"
				}
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": entry.Error})
			},
		}
		proxy.ServeHTTP(out, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
