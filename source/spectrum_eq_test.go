package main

import (
	"testing"
)

// The equalizer mode paints the ring as three bands: bass, middle, treble.
// Each zone must take its own colour and its own level, with no cover tint.
func TestEQModePaintsThreeBandColours(t *testing.T) {
	tap := newSpectrumTap(t.TempDir())
	tap.mu.Lock()
	tap.eqMode = true
	tap.eqColors = [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	tap.eqIntensity = 1
	tap.gain = [3]float64{1, 1, 1}
	tap.state.history = [4]float64{1, 1, 1, 1}
	tap.mu.Unlock()

	frame := tap.rawFrame(3, 200)
	if len(frame) != 3 {
		t.Fatalf("frame has %d zones", len(frame))
	}
	for i, want := range [3]string{"red", "green", "blue"} {
		r, g, b := parseHex(t, frame[i])
		switch want {
		case "red":
			if !(r > g && r > b) {
				t.Fatalf("bass zone %q is not red", frame[i])
			}
		case "green":
			if !(g > r && g > b) {
				t.Fatalf("middle zone %q is not green", frame[i])
			}
		case "blue":
			if !(b > r && b > g) {
				t.Fatalf("treble zone %q is not blue", frame[i])
			}
		}
	}
}

// A silent band must stay dark while its neighbours are lit.
func TestEQModeFollowsEachBand(t *testing.T) {
	tap := newSpectrumTap(t.TempDir())
	tap.mu.Lock()
	tap.eqMode = true
	tap.eqColors = [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	tap.eqIntensity = 1
	tap.gain = [3]float64{1, 1, 1}
	tap.state.history = [4]float64{0, 1, 0, 0}
	tap.mu.Unlock()

	frame := tap.rawFrame(3, 200)
	// A silent band keeps only the dim visibility floor, far below a live one.
	bassR, _, _ := parseHex(t, frame[0])
	_, midG, _ := parseHex(t, frame[1])
	trebleR, _, trebleB := parseHex(t, frame[2])
	if midG < 100 {
		t.Fatalf("middle band is dark: %q", frame[1])
	}
	if bassR >= midG/2 {
		t.Fatalf("silent bass is as bright as the middle band: %q", frame[0])
	}
	if trebleR+trebleB >= midG/2 {
		t.Fatalf("silent treble is as bright as the middle band: %q", frame[2])
	}
}
