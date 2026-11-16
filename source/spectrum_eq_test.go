package main

import (
	"math"
	"testing"
)

// The equalizer keeps one colour for the whole ring and changes only the
// brightness of three band zones, like a level meter.
func TestEQModeVariesOnlyBrightness(t *testing.T) {
	tap := newSpectrumTap(t.TempDir())
	tap.mu.Lock()
	tap.eqMode = true
	tap.eqIntensity = 1
	tap.gain = [3]float64{1, 1, 1}
	tap.state.history = [4]float64{1, 1, 1, 1}
	tap.mu.Unlock()

	frame := tap.rawFrame(3, 200)
	if len(frame) != 3 {
		t.Fatalf("frame has %d zones", len(frame))
	}
	var ratio [2]float64
	for i, s := range frame {
		r, g, b := parseHex(t, s)
		if r == 0 {
			t.Fatalf("zone %d is dark: %q", i, s)
		}
		if i == 0 {
			ratio = [2]float64{float64(g) / float64(r), float64(b) / float64(r)}
			continue
		}
		if math.Abs(float64(g)/float64(r)-ratio[0]) > .03 || math.Abs(float64(b)/float64(r)-ratio[1]) > .03 {
			t.Fatalf("zone %d changed colour: %q vs %q", i, s, frame[0])
		}
	}
}

// Each band lights its own zone; the other zones keep only the dim floor.
func TestEQModeFollowsEachBand(t *testing.T) {
	for band := 0; band < 3; band++ {
		tap := newSpectrumTap(t.TempDir())
		tap.mu.Lock()
		tap.eqMode = true
		tap.eqIntensity = 1
		tap.gain = [3]float64{1, 1, 1}
		var history [4]float64
		switch band {
		case 0:
			history[0] = 1
		case 1:
			history[1], history[2] = 1, 1
		case 2:
			history[3] = 1
		}
		tap.state.history = history
		tap.mu.Unlock()

		frame := tap.rawFrame(6, 200)
		lit := -1
		for i := range frame {
			zone := i * 3 / len(frame)
			if zone > 2 {
				zone = 2
			}
			if zone == band {
				if _, g, _ := parseHex(t, frame[i]); g > lit {
					lit = g
				}
			}
		}
		if lit < 80 {
			t.Fatalf("band %d zone is dark", band)
		}
		for i := range frame {
			zone := i * 3 / len(frame)
			if zone > 2 {
				zone = 2
			}
			if zone == band {
				continue
			}
			if _, g, _ := parseHex(t, frame[i]); g >= lit/2 {
				t.Fatalf("band %d lit a foreign zone at LED %d: %q", band, i, frame[i])
			}
		}
	}
}

// Choosing the equalizer must animate even if the old "Спектр PCM" checkbox is
// off: the equalizer is drawn by the spectrum renderer.
func TestEqualizerModeImpliesSpectrum(t *testing.T) {
	off := false
	cfg := controlsConfig{LEDs: true, Spectrum: &off, SpectrumMode: "eq"}
	if !cfg.useSpectrum() {
		t.Fatal("eq mode did not enable the spectrum renderer")
	}
	cfg.SpectrumMode = "palette"
	if cfg.useSpectrum() {
		t.Fatal("palette mode ignored the disabled spectrum checkbox")
	}
}
