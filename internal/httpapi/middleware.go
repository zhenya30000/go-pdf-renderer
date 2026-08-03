package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	count, err := r.ResponseWriter.Write(data)
	r.bytes += count
	return count, err
}

func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		started := time.Now()
		w.Header().Set("X-Request-ID", middleware.GetReqID(request.Context()))
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, request)
		a.logger.Info("http request",
			"request_id", middleware.GetReqID(request.Context()),
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
			"bytes", recorder.bytes,
			"duration", time.Since(started),
		)
	})
}
