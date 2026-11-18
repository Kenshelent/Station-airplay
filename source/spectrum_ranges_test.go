package main

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func rangeTone(rate int, hz float64, opposite bool) []byte {
	pcm := make([]byte, rate/3*4)
	for i := 0; i < len(pcm)/4; i++ {
		l := int16(8000 * math.Sin(2*math.Pi*hz*float64(i)/float64(rate)))
		r := l
		if opposite {
			r = -l
		}
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(l))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(r))
	}
	return pcm
}

func TestRangesCoverCompleteAudioSpectrum(t *testing.T) {
	for _, rate := range []int{44100, 48000, 96000} {
		for band, hz := range []float64{60, 120, 250, 500, 1000, 2000, 4000, 8000} {
			s := &airplayState{sampleRate: rate}
			s.audio(rangeTone(rate, hz, false), time.Now())
			best := 0
			for i, value := range s.bands {
				if value > s.bands[best] {
					best = i
				}
			}
			if best != band || s.bands[band] < .5 {
				t.Fatalf("rate=%d tone=%.0f expected band=%d got=%v", rate, hz, band, s.bands)
			}
		}
	}
}

func TestRangesDoNotCancelOppositeStereo(t *testing.T) {
	for _, hz := range []float64{80, 850, 8500} {
		mono, opposite := &airplayState{sampleRate: 48000}, &airplayState{sampleRate: 48000}
		mono.audio(rangeTone(48000, hz, false), time.Now())
		opposite.audio(rangeTone(48000, hz, true), time.Now())
		for i := range mono.bands {
			if math.Abs(mono.bands[i]-opposite.bands[i]) > 1e-9 {
				t.Fatalf("%.0f Hz phase cancelled band %d: %v vs %v", hz, i, mono.bands, opposite.bands)
			}
		}
	}
}

func TestRangesTrebleDoesNotAliasIntoBassOrMids(t *testing.T) {
	for _, rate := range []int{44100, 48000, 96000} {
		s := &airplayState{sampleRate: rate}
		s.audio(rangeTone(rate, 8000, false), time.Now())
		if s.bands[7] < .5 {
			t.Fatalf("8 kHz missing at %d: %v", rate, s.bands)
		}
		for i := 0; i < 6; i++ {
			if s.bands[i] > .05 {
				t.Fatalf("8 kHz aliased at %d into %d: %v", rate, i, s.bands)
			}
		}
	}
}

func TestRangesPCMReachesBassMiddleTrebleLEDZones(t *testing.T) {
	for zone, hz := range []float64{80, 850, 8500} {
		for _, rate := range []int{44100, 48000, 96000} {
			tap := newSpectrumTap(t.TempDir())
			tap.state.sampleRate = rate
			tap.eqMode, tap.eqIntensity = true, 1
			tap.gain = [3]float64{1, 1, 1}
			tap.coverColor, tap.haveCoverColor = [3]float64{1, 1, 1}, true
			tap.feedAudio(rangeTone(rate, hz, false), time.Now())
			frame := tap.rawFrame(6, 200)
			lit, _, _ := parseHex(t, frame[zone*2])
			if lit < 100 {
				t.Fatalf("%.0f Hz at %d did not reach zone %d: %v", hz, rate, zone, frame)
			}
			for i, color := range frame {
				level, _, _ := parseHex(t, color)
				if i/2 != zone && level > lit/2 {
					t.Fatalf("%.0f Hz at %d lights unrelated zone %d: %v", hz, rate, i/2, frame)
				}
			}
		}
	}
}

func TestRangesAllEightBandsReachSixLEDs(t *testing.T) {
	for band := 0; band < 8; band++ {
		tap := newSpectrumTap(t.TempDir())
		tap.gain = [3]float64{1, 1, 1}
		tap.coverColor, tap.haveCoverColor = [3]float64{1, 1, 1}, true
		tap.state.bands[band] = 1
		frame := tap.rawFrame(6, 200)
		lit := false
		for _, color := range frame {
			r, _, _ := parseHex(t, color)
			lit = lit || r > 150
		}
		if !lit {
			t.Fatalf("band %d dropped by six-LED mapping: %v", band, frame)
		}
	}
}
