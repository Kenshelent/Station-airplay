package main

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

// A tone must light its own octave band, not the whole spectrum.
func TestSpectrumBandsFollowTone(t *testing.T) {
	cases := []struct {
		hz   float64
		band int
	}{
		{80, 0}, {440, 3}, {3000, 6},
	}
	for _, c := range cases {
		state := &airplayState{sampleRate: 48000}
		// Two seconds of a pure tone at -12 dBFS.
		samples := 48000 * 2
		data := make([]byte, samples*4)
		for i := 0; i < samples; i++ {
			v := int16(8000 * math.Sin(2*math.Pi*c.hz*float64(i)/48000))
			binary.LittleEndian.PutUint16(data[i*4:], uint16(v))
			binary.LittleEndian.PutUint16(data[i*4+2:], uint16(v))
		}
		state.audio(data, time.Now())
		best, bestValue := 0, -1.0
		for i, value := range state.bands {
			if value > bestValue {
				best, bestValue = i, value
			}
		}
		if best != c.band && best != c.band-1 && best != c.band+1 {
			t.Fatalf("%.0f Hz lit band %d (%v), want %d", c.hz, best, state.bands, c.band)
		}
		if bestValue < .2 {
			t.Fatalf("%.0f Hz too quiet: %v", c.hz, state.bands)
		}
	}
}

// Silence must not leave frozen bars: every band decays to zero.
func TestSpectrumBandsDecayToSilence(t *testing.T) {
	state := &airplayState{sampleRate: 48000}
	loud := make([]byte, 48000*4)
	for i := 0; i < 48000; i++ {
		v := int16(12000 * math.Sin(2*math.Pi*1000*float64(i)/48000))
		binary.LittleEndian.PutUint16(loud[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(loud[i*4+2:], uint16(v))
	}
	state.audio(loud, time.Now())
	if state.bands[4] < .1 {
		t.Fatalf("tone was not measured: %v", state.bands)
	}
	silence := make([]byte, 48000*4)
	for i := 0; i < 30; i++ {
		state.audio(silence, time.Now())
	}
	for i, value := range state.bands {
		if value > .02 {
			t.Fatalf("band %d stuck at %.3f", i, value)
		}
	}
}

// The animation runs at 20 fps and eases between frames instead of flashing.
func TestVisualizerFrameRateAndCrossFade(t *testing.T) {
	if animationInterval != 50*time.Millisecond {
		t.Fatalf("frame interval %s", animationInterval)
	}
	shown := []string{"#000000", "#102030"}
	next := []string{"#FFFFFF", "#FFFFFF"}
	mid := blendFrames(shown, next, .5)
	if mid[0] != "#7F7F7F" && mid[0] != "#808080" {
		t.Fatalf("mid frame %q", mid[0])
	}
	// The blend must approach the target, not oscillate around it.
	for i := 0; i < 10; i++ {
		mid = blendFrames(mid, next, .45)
	}
	if mid[0] != "#FFFFFF" && mid[0] != "#FEFEFE" {
		t.Fatalf("blend did not converge: %q", mid[0])
	}
	if got := blendFrames([]string{"#000000"}, next, .5); len(got) != 2 {
		t.Fatalf("length mismatch not handled: %v", got)
	}
	if r, g, b, ok := parseLedColor("#A1B2C3"); !ok || r != 0xA1 || g != 0xB2 || b != 0xC3 {
		t.Fatalf("parse %d %d %d %v", r, g, b, ok)
	}
	if _, _, _, ok := parseLedColor("nope"); ok {
		t.Fatal("invalid colour accepted")
	}
}
