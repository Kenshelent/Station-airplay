package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMiniCandidateTickScope(t *testing.T) {
	for _, model := range []string{"yandexmini", "YandexStationMini", "yandexmini_2", "plum", "yandexplum", "yandexstation_2", "pickle", ""} {
		want := 500 * time.Millisecond
		if model == "yandexmini" || model == "YandexStationMini" {
			want = 250 * time.Millisecond
		}
		if got := controlsTickInterval(yamDevice{Model: model}); got != want {
			t.Fatalf("%s: tick %v, want %v", model, got, want)
		}
	}
}

func TestMiniCandidateBackendScope(t *testing.T) {
	fixture, _ := amixerFixture(t, "20", true)
	t.Setenv("PATH", filepath.Dir(fixture.path)+":"+os.Getenv("PATH"))
	probe := volumeManagerListening
	t.Cleanup(func() { volumeManagerListening = probe })
	for _, online := range []bool{false, true} {
		volumeManagerListening = func() bool { return online }
		for _, model := range []string{"yandexmini", "YandexStationMini", "yandexmini_2", "plum", "yandexplum", "yandexstation_2", "pickle", ""} {
			host, _, _ := lunaVolumeHost(t, 20)
			api := lunaVolumeAPI(t, host)
			v, err := controlsVolume(context.Background(), api, yamDevice{Model: model}, controlsConfig{NativeVolume: true})
			if err != nil {
				t.Fatal(err)
			}
			_, fast := v.(*miniMixerWriteVolume)
			if want := originalMini(yamDevice{Model: model}) && !online; fast != want {
				t.Fatalf("%s online=%v backend=%T", model, online, v)
			}
		}
	}
}

func TestMiniCandidateMixerWriteAndLunaRead(t *testing.T) {
	mixer, _ := amixerFixture(t, "20", true)
	reader := &stubVolume{get: 27}
	v := &miniMixerWriteVolume{reader: reader, mixer: mixer}
	if got, err := v.SetVolume(context.Background(), 40); err != nil || got != 40 {
		t.Fatalf("write %v %v", got, err)
	}
	if len(reader.sets) != 0 {
		t.Fatal("successful ALSA write must not also write Luna")
	}
	if got, err := v.GetVolume(context.Background()); err != nil || got != 27 {
		t.Fatalf("read must reflect Luna, not mask stale logical state: %v %v", got, err)
	}
	mixer.luna = reader
	mixer.path = "/nonexistent-mini-amixer"
	if _, err := v.SetVolume(context.Background(), 41); err != nil || len(reader.sets) != 1 {
		t.Fatalf("failed mixer must fall back to Luna: %v", err)
	}
}

func TestMiniCandidateDarkSolidColours(t *testing.T) {
	for _, tt := range []struct {
		rgb  [3]float64
		want string
	}{
		{[3]float64{1.0 / 255, 0, 0}, "#FF0000"},
		{[3]float64{0, 10.0 / 255, 0}, "#00FF00"},
		{[3]float64{0, 0, 19.0 / 255}, "#0000FF"},
		{[3]float64{10.0 / 255, 10.0 / 255, 10.0 / 255}, "#FFFFFF"},
		{[3]float64{}, "#000000"},
	} {
		tap := newSpectrumTap(t.TempDir())
		tap.coverColor, tap.haveCoverColor = tt.rgb, true
		tap.neutralBlack, tap.solidFloor = true, solidFullValue
		for _, got := range tap.rawSolidFrame(6, 200) {
			if got != tt.want {
				t.Fatalf("%v: got %s, want %s", tt.rgb, got, tt.want)
			}
		}
	}
	// Lite 2 / Station 2 near-black policy must not change.
	rgb := [3]float64{10.0 / 255, 0, 0}
	if !solidBlackArtwork(rgb, solidMinValue) || solidBlackArtwork(rgb, solidFullValue) {
		t.Fatal("near-black threshold scope changed")
	}
}
