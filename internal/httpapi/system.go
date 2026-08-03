package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type metrics struct {
	requests                  atomic.Uint64
	successes                 atomic.Uint64
	errors                    atomic.Uint64
	timeouts                  atomic.Uint64
	inFlight                  atomic.Int64
	renderDurationNanoseconds atomic.Uint64
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), a.readyTimeout)
	defer cancel()
	if err := a.renderer.Ready(ctx); err != nil {
		a.writeProblem(w, request, http.StatusServiceUnavailable, "Not Ready", "Chrome is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) versionInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    a.version,
		"commit":     a.commit,
		"build_date": a.buildDate,
	})
}

func (a *API) prometheusMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "go_pdf_renderer_requests_total %d\n", a.metrics.requests.Load())
	_, _ = fmt.Fprintf(w, "go_pdf_renderer_successes_total %d\n", a.metrics.successes.Load())
	_, _ = fmt.Fprintf(w, "go_pdf_renderer_errors_total %d\n", a.metrics.errors.Load())
	_, _ = fmt.Fprintf(w, "go_pdf_renderer_timeouts_total %d\n", a.metrics.timeouts.Load())
	_, _ = fmt.Fprintf(w, "go_pdf_renderer_renders_in_flight %d\n", a.metrics.inFlight.Load())
	_, _ = fmt.Fprintf(w, "go_pdf_renderer_render_duration_seconds_total %f\n", float64(a.metrics.renderDurationNanoseconds.Load())/float64(time.Second))
}

func (a *API) openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(a.openAPISpec)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
