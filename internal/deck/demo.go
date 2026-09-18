package deck

import (
	"fmt"
	"net/http"
	"time"
)

func DemoHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/orders", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"orders": []map[string]any{
			{"id": "ORD-1042", "customer": "Alex Morgan", "item": "Mechanical keyboard", "total": 129, "status": "shipped"},
			{"id": "ORD-1043", "customer": "Jamie Chen", "item": "Desk light", "total": 49, "status": "processing"},
			{"id": "ORD-1044", "customer": "Sam Taylor", "item": "USB-C hub", "total": 79, "status": "delivered"},
		}, "source": "FaultDeck built-in demo", "synthetic": true})
	})
	mux.HandleFunc("GET /api/products", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"products": []map[string]any{{"id": "p-1", "name": "Mechanical keyboard", "price": 129}, {"id": "p-2", "name": "Desk light", "price": 49}}, "synthetic": true})
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "healthy", "service": "faultdeck-demo"})
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		for i := 1; i <= 5; i++ {
			if i > 1 && !waitFor(r.Context(), 100) {
				return
			}
			_, _ = fmt.Fprintf(w, "id: %d\nevent: heartbeat\ndata: {\"tick\":%d}\n\n", i, i)
			if err := http.NewResponseController(w).Flush(); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"service": "FaultDeck demo", "endpoints": []string{"/api/orders", "/api/products", "/api/health", "/api/events"}, "time": time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]string{"error": "demo endpoint not found"})
	})
	return mux
}
