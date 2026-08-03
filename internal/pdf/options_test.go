package pdf

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOptionsNormalizeDefaults(t *testing.T) {
	options, err := (Options{}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if options.Format != "A4" || options.WidthMM != 210 || options.HeightMM != 297 {
		t.Fatalf("unexpected page defaults: %+v", options)
	}
	if options.Scale != 1 || options.WaitUntil != "assets" {
		t.Fatalf("unexpected render defaults: %+v", options)
	}
	if !*options.PrintBackground || !*options.PreferCSSPageSize {
		t.Fatalf("unexpected print defaults: %+v", options)
	}
}

func TestOptionsNormalizeCustom(t *testing.T) {
	background := false
	cssSize := false
	options, err := (Options{
		Format:            "custom",
		WidthMM:           100,
		HeightMM:          150,
		Scale:             1.5,
		PrintBackground:   &background,
		PreferCSSPageSize: &cssSize,
		WaitUntil:         "fonts",
		RenderDelay:       Duration(250 * time.Millisecond),
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if options.Format != "CUSTOM" || options.WidthMM != 100 || options.HeightMM != 150 {
		t.Fatalf("unexpected custom options: %+v", options)
	}
	if *options.PrintBackground || *options.PreferCSSPageSize {
		t.Fatalf("explicit false values were lost: %+v", options)
	}
}

func TestOptionsNormalizeRejectsInvalidValues(t *testing.T) {
	tests := []Options{
		{Format: "A3"},
		{Format: "custom", WidthMM: 9, HeightMM: 100},
		{Scale: 2.1},
		{Margins: Margins{TopMM: -1}},
		{PageRanges: "3-1 nope"},
		{WaitUntil: "network"},
		{RenderDelay: Duration(6 * time.Second)},
	}
	for index, options := range tests {
		if _, err := options.Normalize(); err == nil {
			t.Fatalf("case %d: expected error", index)
		}
	}
}

func TestDurationJSON(t *testing.T) {
	var value struct {
		Delay Duration `json:"delay"`
	}
	if err := json.Unmarshal([]byte(`{"delay":"750ms"}`), &value); err != nil {
		t.Fatal(err)
	}
	if time.Duration(value.Delay) != 750*time.Millisecond {
		t.Fatalf("unexpected duration: %s", time.Duration(value.Delay))
	}
}
