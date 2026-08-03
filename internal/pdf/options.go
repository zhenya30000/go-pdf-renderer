package pdf

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var pageRangesPattern = regexp.MustCompile(`^\s*\d+(?:\s*-\s*\d+)?(?:\s*,\s*\d+(?:\s*-\s*\d+)?)*\s*$`)

type Request struct {
	HTML    string  `json:"html"`
	Options Options `json:"options,omitempty"`
}

type Options struct {
	Format            string   `json:"format,omitempty"`
	WidthMM           float64  `json:"width_mm,omitempty"`
	HeightMM          float64  `json:"height_mm,omitempty"`
	Landscape         bool     `json:"landscape,omitempty"`
	Margins           Margins  `json:"margins,omitempty"`
	Scale             float64  `json:"scale,omitempty"`
	PrintBackground   *bool    `json:"print_background,omitempty"`
	PreferCSSPageSize *bool    `json:"prefer_css_page_size,omitempty"`
	HeaderTemplate    string   `json:"header_template,omitempty"`
	FooterTemplate    string   `json:"footer_template,omitempty"`
	PageRanges        string   `json:"page_ranges,omitempty"`
	TaggedPDF         bool     `json:"tagged_pdf,omitempty"`
	DocumentOutline   bool     `json:"document_outline,omitempty"`
	WaitUntil         string   `json:"wait_until,omitempty"`
	RenderDelay       Duration `json:"render_delay,omitempty"`
}

type Margins struct {
	TopMM    float64 `json:"top_mm,omitempty"`
	RightMM  float64 `json:"right_mm,omitempty"`
	BottomMM float64 `json:"bottom_mm,omitempty"`
	LeftMM   float64 `json:"left_mm,omitempty"`
}

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), `"`)
	if raw == "" || raw == "null" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	*d = Duration(parsed)
	return nil
}

func (o Options) Normalize() (Options, error) {
	o.Format = strings.ToUpper(strings.TrimSpace(o.Format))
	if o.Format == "" {
		o.Format = "A4"
	}
	if o.WaitUntil == "" {
		o.WaitUntil = "assets"
	}
	o.WaitUntil = strings.ToLower(strings.TrimSpace(o.WaitUntil))
	if o.Scale == 0 {
		o.Scale = 1
	}
	if o.PrintBackground == nil {
		value := true
		o.PrintBackground = &value
	}
	if o.PreferCSSPageSize == nil {
		value := true
		o.PreferCSSPageSize = &value
	}

	switch o.Format {
	case "A4":
		o.WidthMM = 210
		o.HeightMM = 297
	case "LETTER":
		o.WidthMM = 215.9
		o.HeightMM = 279.4
	case "LEGAL":
		o.WidthMM = 215.9
		o.HeightMM = 355.6
	case "CUSTOM":
		if err := dimension("width_mm", o.WidthMM); err != nil {
			return Options{}, err
		}
		if err := dimension("height_mm", o.HeightMM); err != nil {
			return Options{}, err
		}
	default:
		return Options{}, fmt.Errorf("format must be A4, Letter, Legal, or Custom")
	}

	if o.Scale < 0.1 || o.Scale > 2 {
		return Options{}, errors.New("scale must be between 0.1 and 2")
	}
	for name, margin := range map[string]float64{
		"top_mm": o.Margins.TopMM, "right_mm": o.Margins.RightMM,
		"bottom_mm": o.Margins.BottomMM, "left_mm": o.Margins.LeftMM,
	} {
		if margin < 0 || margin > 100 {
			return Options{}, fmt.Errorf("%s must be between 0 and 100", name)
		}
	}
	if len(o.HeaderTemplate) > 64<<10 || len(o.FooterTemplate) > 64<<10 {
		return Options{}, errors.New("header and footer templates must not exceed 64 KiB")
	}
	if o.PageRanges != "" && !pageRangesPattern.MatchString(o.PageRanges) {
		return Options{}, errors.New("page_ranges has invalid format")
	}
	if o.WaitUntil != "none" && o.WaitUntil != "fonts" && o.WaitUntil != "assets" {
		return Options{}, errors.New("wait_until must be none, fonts, or assets")
	}
	delay := time.Duration(o.RenderDelay)
	if delay < 0 || delay > 5*time.Second {
		return Options{}, errors.New("render_delay must be between 0s and 5s")
	}
	return o, nil
}

func dimension(name string, value float64) error {
	if value < 10 || value > 2000 {
		return fmt.Errorf("%s must be between 10 and 2000", name)
	}
	return nil
}

func mmToInches(value float64) float64 {
	return value / 25.4
}
