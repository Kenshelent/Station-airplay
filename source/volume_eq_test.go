package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMixerControlFollowsFirmwareSoftvol(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "asound.conf")
	original := asoundConfigPath
	defer func() { asoundConfigPath = original }()
	asoundConfigPath = path

	if err := os.WriteFile(path, []byte(`pcm.dmixer { type dmix }
pcm.softvol {
    type softvol
    slave.pcm "hw:0,2"
    control.name "Softmaster"
    control.card 0
}
pcm.other { control.name "Ignored" }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := mixerControl(); got != "Softmaster" {
		t.Fatalf("control %q, want Softmaster", got)
	}
	if err := os.WriteFile(path, []byte(`pcm.softvol { control { name "Master"; card 0; } }`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := mixerControl(); got != "Master" {
		t.Fatalf("control %q, want Master", got)
	}
	asoundConfigPath = filepath.Join(dir, "missing.conf")
	if got := mixerControl(); got != "Master" {
		t.Fatalf("missing config fell back to %q", got)
	}
}

func TestEqualizerKeepsLoudness(t *testing.T) {
	const rate = 48000
	frames := rate / 2
	input := make([]byte, frames*4)
	for i := 0; i < frames; i++ {
		// 100 Hz sine at about -12 dBFS: inside the boosted bass band.
		v := int16(8000 * math.Sin(2*math.Pi*100*float64(i)/rate))
		binary.LittleEndian.PutUint16(input[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(input[i*4+2:], uint16(v))
	}
	cfg := equalizerConfig{Enabled: true, BandsDB: [5]float64{12, 4, 1, 1, 4}}
	var out bytes.Buffer
	if err := processEqualizer(bytes.NewReader(input), &out, cfg, rate); err != nil {
		t.Fatal(err)
	}
	rms := func(data []byte) float64 {
		var sum float64
		for i := 0; i+1 < len(data); i += 2 {
			x := float64(int16(binary.LittleEndian.Uint16(data[i:])))
			sum += x * x
		}
		return math.Sqrt(sum / float64(len(data)/2))
	}
	ratio := 20 * math.Log10(rms(out.Bytes())/rms(input))
	// A boosting EQ must not turn the stream into a whisper, and must not
	// double it either; anything inside +/-6 dB is a normal tonal change.
	if ratio < -6 || ratio > 6 {
		t.Fatalf("equalizer changed loudness by %.1f dB", ratio)
	}
	// Peaks stay inside the S16 range after the soft limiter.
	for i := 0; i+1 < out.Len(); i += 2 {
		if v := int16(binary.LittleEndian.Uint16(out.Bytes()[i:])); v == math.MaxInt16 || v == math.MinInt16 {
			t.Fatalf("hard clip at sample %d", i/2)
		}
	}
}
