package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestEqualizerBypassIsBitExact(t *testing.T) {
	input := make([]byte, 4097)
	for i := range input {
		input[i] = byte(i)
	}
	var out bytes.Buffer
	if err := processEqualizer(bytes.NewReader(input), &out, equalizerConfig{}, 48000); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, out.Bytes()) {
		t.Fatal("bypass altered bytes")
	}
}
func TestEqualizerFlatAndStereo(t *testing.T) {
	input := make([]byte, 48000*4)
	for i := 0; i < 48000; i++ {
		binary.LittleEndian.PutUint16(input[i*4:], uint16(int16(10000*math.Sin(float64(i)*2*math.Pi*1000/48000))))
	}
	var out bytes.Buffer
	if err := processEqualizer(bytes.NewReader(input), &out, equalizerConfig{Enabled: true}, 48000); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, out.Bytes()) {
		t.Fatal("flat EQ changed signal or leaked channels")
	}
}
func TestEqualizerCutResponse(t *testing.T) {
	for _, rate := range []int{44100, 48000} {
		input := make([]byte, rate*4)
		for i := 0; i < rate; i++ {
			binary.LittleEndian.PutUint16(input[i*4:], uint16(int16(10000*math.Sin(float64(i)*2*math.Pi*1000/float64(rate)))))
		}
		var out bytes.Buffer
		cfg := equalizerConfig{Enabled: true}
		cfg.BandsDB[2] = -12
		if err := processEqualizer(bytes.NewReader(input), &out, cfg, rate); err != nil {
			t.Fatal(err)
		}
		var before, after float64
		for i := rate / 2; i < rate; i++ {
			a := float64(int16(binary.LittleEndian.Uint16(input[i*4:])))
			b := float64(int16(binary.LittleEndian.Uint16(out.Bytes()[i*4:])))
			before += a * a
			after += b * b
			if binary.LittleEndian.Uint16(out.Bytes()[i*4+2:]) != 0 {
				t.Fatal("stereo leaked")
			}
		}
		db := 10 * math.Log10(after/before)
		if math.Abs(db+12) > 0.1 {
			t.Fatal("incorrect cut", rate, db)
		}
	}
}
func TestEqualizerValidationAndTruncatedInput(t *testing.T) {
	if err := (&equalizerConfig{PreampDB: 1}).validate(); err == nil {
		t.Fatal("positive preamp")
	}
	if err := processEqualizer(bytes.NewReader([]byte{0}), &bytes.Buffer{}, equalizerConfig{Enabled: true}, 48000); err == nil {
		t.Fatal("truncated frame")
	}
}

func TestEqualizerRejectsWrongBandCount(t *testing.T) {
	for _, data := range []string{`{"bands_db":[1]}`, `{"bands_db":[1,2,3,4,5,6]}`, `{"enabled":true,"unknown":1}`} {
		var config equalizerConfig
		if err := json.Unmarshal([]byte(data), &config); err == nil {
			t.Fatal(data)
		}
	}
}

func TestEqualizerPositiveGainsReserveHeadroom(t *testing.T) {
	input := make([]byte, 48000*4)
	for i := 0; i < 48000; i++ {
		value := uint16(int16(32000 * math.Sin(float64(i)*2*math.Pi*1000/48000)))
		binary.LittleEndian.PutUint16(input[i*4:], value)
		binary.LittleEndian.PutUint16(input[i*4+2:], value)
	}
	var out bytes.Buffer
	if err := processEqualizer(bytes.NewReader(input), &out, equalizerConfig{Enabled: true, BandsDB: [5]float64{12, 12, 12, 12, 12}}, 48000); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(out.Bytes()); i += 2 {
		value := int16(binary.LittleEndian.Uint16(out.Bytes()[i:]))
		if value == 32767 || value == -32768 {
			t.Fatal("clipped")
		}
	}
}

func TestEqualizerAsoundQuotesPaths(t *testing.T) {
	config := eqAsound("/a b/receiver", "/x'y/config", "plug:dmix_spk", 48000)
	if !strings.Contains(config, "--eq-worker") || !strings.Contains(config, "slave.pcm \"null\"") {
		t.Fatal(config)
	}
}

func BenchmarkEqualizer(b *testing.B) {
	input := make([]byte, 48000*4)
	for n := 0; n < b.N; n++ {
		if err := processEqualizer(bytes.NewReader(input), &bytes.Buffer{}, equalizerConfig{Enabled: true}, 48000); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(input)))
}
