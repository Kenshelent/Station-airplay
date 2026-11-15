package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestColorLUTDefaultsStayIdentity(t *testing.T) {
	if (&spectrumPoints{}).lut() != nil {
		t.Fatal("empty points must keep the fast path")
	}
	lut := (&spectrumPoints{Grey: "#808080"}).lut()
	if lut == nil {
		t.Fatal("one point must build a table")
	}
	for r := 0; r < 3; r++ {
		for g := 0; g < 3; g++ {
			for b := 0; b < 3; b++ {
				in := [3]float64{float64(r) / 2, float64(g) / 2, float64(b) / 2}
				got := lut.mapColor(in)
				if got != in {
					if r == 1 && g == 1 && b == 1 {
						continue // the grey node itself was edited
					}
					t.Fatalf("node %d/%d/%d moved: %v", r, g, b, got)
				}
			}
		}
	}
}

func TestColorLUTExactNodes(t *testing.T) {
	points := &spectrumPoints{
		Grey:    "#112233",
		Red:     "#223344",
		Green:   "#334455",
		Blue:    "#445566",
		Yellow:  "#556677",
		Cyan:    "#667788",
		Magenta: "#778899",
		Orange:  "#8899AA",
		White:   "#99AABB",
	}
	lut := points.lut()
	for _, tc := range []struct {
		name string
		in   [3]float64
		hex  string
	}{
		{"grey", [3]float64{.5, .5, .5}, "#112233"},
		{"red", [3]float64{1, 0, 0}, "#223344"},
		{"green", [3]float64{0, 1, 0}, "#334455"},
		{"blue", [3]float64{0, 0, 1}, "#445566"},
		{"yellow", [3]float64{1, 1, 0}, "#556677"},
		{"cyan", [3]float64{0, 1, 1}, "#667788"},
		{"magenta", [3]float64{1, 0, 1}, "#778899"},
		{"orange", [3]float64{1, .5, 0}, "#8899AA"},
		{"white", [3]float64{1, 1, 1}, "#99AABB"},
	} {
		v, err := parsePointHex(tc.hex)
		if err != nil {
			t.Fatal(err)
		}
		got := lut.mapColor(tc.in)
		for ch := range got {
			if math.Abs(got[ch]-v[ch]) > 1e-9 {
				t.Fatalf("%s: got %v want %v", tc.name, got, v)
			}
		}
	}
}

func TestColorLUTInterpolatesBetweenNodes(t *testing.T) {
	// The table has nodes at 0, 0.5 and 1. A point halfway between the
	// midpoint and the calibrated red node must sit halfway between their
	// values, not at the uncalibrated identity.
	lut := (&spectrumPoints{Red: "#400000"}).lut()
	got := lut.mapColor([3]float64{.75, 0, 0})
	want := [3]float64{(0.5 + 64.0/255) / 2, 0, 0}
	for ch := range got {
		if math.Abs(got[ch]-want[ch]) > 1e-9 {
			t.Fatalf("channel %d: got %.9f want %.9f", ch, got[ch], want[ch])
		}
	}
}

func TestSpectrumPointsValidation(t *testing.T) {
	defaults := defaultSpectrumPoints()
	if err := defaults.validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&spectrumPoints{Orange: "orange"}).validate(); err == nil {
		t.Fatal("invalid hex accepted")
	}
	if err := (&spectrumPoints{White: "#FFFFFF"}).validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadControlsSpectrumPoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controls.json")
	data := []byte(`{"spectrum_points":{"orange":"#123456"},"defaults_version":3}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadControls(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpectrumPoints == nil || cfg.SpectrumPoints.Orange != "#123456" {
		t.Fatalf("points not loaded: %+v", cfg.SpectrumPoints)
	}
	lut := cfg.SpectrumPoints.lut()
	v, _ := parsePointHex("#123456")
	got := lut.mapColor([3]float64{1, .5, 0})
	for ch := range got {
		if math.Abs(got[ch]-v[ch]) > 1e-9 {
			t.Fatalf("orange node: got %v want %v", got, v)
		}
	}
	if err := os.WriteFile(path, []byte(`{"spectrum_points":{"orange":"#12345"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadControls(path); err == nil {
		t.Fatal("short hex accepted")
	}
	if err := os.WriteFile(path, []byte(`{"spectrum_points":{"pink":"#FF00FF"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadControls(path); err == nil {
		t.Fatal("unknown point accepted")
	}
}

func parsePointHex(s string) ([3]float64, error) {
	var rgb uint32
	if _, err := fmt.Sscanf(s, "#%06x", &rgb); err != nil {
		return [3]float64{}, err
	}
	return [3]float64{float64(rgb>>16&255) / 255, float64(rgb>>8&255) / 255, float64(rgb&255) / 255}, nil
}
