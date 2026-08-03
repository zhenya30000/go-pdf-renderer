package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address           string
	ChromePath        string
	ChromeNoSandbox   bool
	AllowJavaScript   bool
	AllowedAssetHosts []string
	AllowedOrigins    []string
	APIKey            string
	MaxConcurrent     int
	MaxRequestBytes   int64
	MaxPDFBytes       int
	QueueTimeout      time.Duration
	RenderTimeout     time.Duration
	ReadyTimeout      time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

func Load() (Config, error) {
	port := value("PORT", "3001")
	cfg := Config{
		Address:           value("ADDRESS", ":"+port),
		ChromePath:        strings.TrimSpace(os.Getenv("CHROME_PATH")),
		APIKey:            os.Getenv("API_KEY"),
		AllowedAssetHosts: list("ALLOWED_ASSET_HOSTS"),
		AllowedOrigins:    list("ALLOWED_ORIGINS"),
	}

	var err error
	if cfg.ChromeNoSandbox, err = boolean("CHROME_NO_SANDBOX", false); err != nil {
		return Config{}, err
	}
	if cfg.AllowJavaScript, err = boolean("ALLOW_JAVASCRIPT", false); err != nil {
		return Config{}, err
	}
	if cfg.MaxConcurrent, err = integer("MAX_CONCURRENT_RENDERS", 2, 1, 64); err != nil {
		return Config{}, err
	}
	if cfg.MaxRequestBytes, err = int64Value("MAX_REQUEST_BYTES", 2<<20, 1024, 100<<20); err != nil {
		return Config{}, err
	}
	if cfg.MaxPDFBytes, err = integer("MAX_PDF_BYTES", 20<<20, 1024, 500<<20); err != nil {
		return Config{}, err
	}
	if cfg.QueueTimeout, err = duration("QUEUE_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.RenderTimeout, err = duration("RENDER_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadyTimeout, err = duration("READY_TIMEOUT", 3*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadHeaderTimeout, err = duration("READ_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = duration("READ_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = duration("WRITE_TIMEOUT", 45*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = duration("IDLE_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout <= cfg.RenderTimeout {
		return Config{}, fmt.Errorf("WRITE_TIMEOUT must be greater than RENDER_TIMEOUT")
	}
	return cfg, nil
}

func value(name, fallback string) string {
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
		return raw
	}
	return fallback
}

func list(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	items := strings.Split(raw, ",")
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func boolean(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func integer(name string, fallback, min, max int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return parsed, nil
}

func int64Value(name string, fallback, min, max int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return parsed, nil
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return parsed, nil
}
