package httpapi

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/zhenya3000/pdf-generator/internal/pdf"
)

type Config struct {
	Renderer        pdf.Renderer
	Logger          *slog.Logger
	MaxConcurrent   int
	QueueTimeout    time.Duration
	RenderTimeout   time.Duration
	ReadyTimeout    time.Duration
	MaxRequestBytes int64
	MaxPDFBytes     int
	APIKey          string
	AllowedOrigins  []string
	Version         string
	Commit          string
	BuildDate       string
}

type API struct {
	renderer        pdf.Renderer
	logger          *slog.Logger
	semaphore       chan struct{}
	queueTimeout    time.Duration
	renderTimeout   time.Duration
	readyTimeout    time.Duration
	maxRequestBytes int64
	maxPDFBytes     int
	apiKey          string
	version         string
	commit          string
	buildDate       string
	metrics         metrics
}

type generateRequest struct {
	HTML     string      `json:"html"`
	Filename string      `json:"filename,omitempty"`
	Options  pdf.Options `json:"options,omitempty"`
}

type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func New(cfg Config) http.Handler {
	api := &API{
		renderer:        cfg.Renderer,
		logger:          cfg.Logger,
		semaphore:       make(chan struct{}, cfg.MaxConcurrent),
		queueTimeout:    cfg.QueueTimeout,
		renderTimeout:   cfg.RenderTimeout,
		readyTimeout:    cfg.ReadyTimeout,
		maxRequestBytes: cfg.MaxRequestBytes,
		maxPDFBytes:     cfg.MaxPDFBytes,
		apiKey:          cfg.APIKey,
		version:         cfg.Version,
		commit:          cfg.Commit,
		buildDate:       cfg.BuildDate,
	}
	if api.logger == nil {
		api.logger = slog.Default()
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(api.logRequests)
	if len(cfg.AllowedOrigins) > 0 {
		router.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.AllowedOrigins,
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-API-Key", "X-Request-ID"},
			ExposedHeaders: []string{"Content-Disposition", "Content-Length", "X-Request-ID"},
			MaxAge:         300,
		}))
	}

	router.Get("/healthz", api.health)
	router.Get("/readyz", api.ready)
	router.Get("/version", api.versionInfo)
	router.Get("/metrics", api.prometheusMetrics)
	router.Post("/v1/pdf", api.generate)
	router.Post("/generate-pdf", api.generate)
	return router
}

func (a *API) generate(w http.ResponseWriter, request *http.Request) {
	a.metrics.requests.Add(1)
	if !a.authorized(request) {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusUnauthorized, "Unauthorized", "a valid API key is required")
		return
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "application/json" {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusUnsupportedMediaType, "Unsupported Media Type", "Content-Type must be application/json")
		return
	}

	request.Body = http.MaxBytesReader(w, request.Body, a.maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var body generateRequest
	if err := decoder.Decode(&body); err != nil {
		a.metrics.errors.Add(1)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			a.writeProblem(w, request, http.StatusRequestEntityTooLarge, "Content Too Large", "request body exceeds the configured limit")
			return
		}
		a.writeProblem(w, request, http.StatusBadRequest, "Invalid Request", "request body must contain one valid JSON object")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusBadRequest, "Invalid Request", "request body must contain exactly one JSON object")
		return
	}
	if strings.TrimSpace(body.HTML) == "" {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusBadRequest, "Invalid Request", "html is required")
		return
	}
	filename, err := validFilename(body.Filename)
	if err != nil {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusBadRequest, "Invalid Request", err.Error())
		return
	}
	normalized, err := body.Options.Normalize()
	if err != nil {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusBadRequest, "Invalid Options", err.Error())
		return
	}

	queueTimer := time.NewTimer(a.queueTimeout)
	defer queueTimer.Stop()
	select {
	case a.semaphore <- struct{}{}:
		defer func() { <-a.semaphore }()
	case <-queueTimer.C:
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusServiceUnavailable, "Renderer Busy", "render queue timeout exceeded")
		return
	case <-request.Context().Done():
		a.metrics.errors.Add(1)
		return
	}

	renderContext, cancel := context.WithTimeout(request.Context(), a.renderTimeout)
	defer cancel()
	a.metrics.inFlight.Add(1)
	started := time.Now()
	output, err := a.renderer.Render(renderContext, pdf.Request{HTML: body.HTML, Options: normalized})
	a.metrics.inFlight.Add(-1)
	a.metrics.renderDurationNanoseconds.Add(uint64(time.Since(started)))
	if err != nil {
		a.metrics.errors.Add(1)
		switch {
		case errors.Is(renderContext.Err(), context.DeadlineExceeded):
			a.metrics.timeouts.Add(1)
			a.writeProblem(w, request, http.StatusGatewayTimeout, "Render Timeout", "PDF generation exceeded the configured timeout")
		case errors.Is(renderContext.Err(), context.Canceled):
			return
		default:
			a.logger.Error("render failed", "request_id", middleware.GetReqID(request.Context()), "error", err)
			a.writeProblem(w, request, http.StatusInternalServerError, "Render Failed", "PDF generation failed")
		}
		return
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) || len(output) > a.maxPDFBytes {
		a.metrics.errors.Add(1)
		a.writeProblem(w, request, http.StatusInternalServerError, "Invalid PDF", "generated PDF is empty or exceeds the configured limit")
		return
	}

	a.metrics.successes.Add(1)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(output)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(output); err != nil {
		a.logger.Warn("write response failed", "request_id", middleware.GetReqID(request.Context()), "error", err)
	}
}

func (a *API) authorized(request *http.Request) bool {
	if a.apiKey == "" {
		return true
	}
	provided := request.Header.Get("X-API-Key")
	if provided == "" {
		provided = strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(a.apiKey)) == 1
}

func validFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "document.pdf", nil
	}
	if len(name) > 128 || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return "", errors.New("filename must be a plain name up to 128 bytes")
	}
	for _, value := range name {
		if unicode.IsControl(value) {
			return "", errors.New("filename must not contain control characters")
		}
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name += ".pdf"
	}
	return name, nil
}

func (a *API) writeProblem(w http.ResponseWriter, request *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: request.URL.Path,
	})
}
