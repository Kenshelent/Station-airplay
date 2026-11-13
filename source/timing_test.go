package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimingConfigValidation(t *testing.T) {
	for _, mode := range []string{"", timingSynchronized, timingFastAudio, "fast", "FAST_AUDIO", "-1.7"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "controls.json")
			if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"timing_mode":%q}`, mode)), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := loadControls(path)
			valid := mode == "" || mode == timingSynchronized || mode == timingFastAudio
			if (err == nil) != valid {
				t.Fatalf("mode %q: %v", mode, err)
			}
		})
	}
}

func TestTimingModesAcrossProfiles(t *testing.T) {
	for _, model := range []string{"yandexmini", "yandexmini_2", "yandexstation_2", "yandexmidi", "yandexplum", "plum", "yandexmicro"} {
		p, ok := profileFor(model)
		if !ok {
			t.Fatal(model)
		}
		for _, tc := range []struct {
			mode   string
			offset string
		}{{"", "0.0"}, {timingSynchronized, "0.0"}, {timingFastAudio, "-1.7"}} {
			cfg := &controlsConfig{TimingMode: tc.mode}
			for _, conf := range []string{
				receiverControlConfig(p, cfg),
				// Optional controls becoming unavailable must not reset timing.
				receiverControlConfig(profileWithTiming(p, cfg), nil),
			} {
				key := "audio_backend_latency_offset_in_seconds = "
				if !strings.Contains(conf, key+tc.offset+";") || strings.Count(conf, key) != 1 {
					t.Fatalf("%s/%s: incorrect offset: %s", model, tc.mode, conf)
				}
				if !strings.Contains(conf, "audio_backend_buffer_desired_length_in_seconds = 0.10;") {
					t.Fatal("timing mode changed backend buffering")
				}
			}
		}
	}
}

func TestTimingSurvivesDefaultsMigration(t *testing.T) {
	for _, mode := range []string{"", timingSynchronized, timingFastAudio} {
		path, cfg := writeControlsFile(t, fmt.Sprintf(`{"timing_mode":%q,"leds":false}`, mode))
		out, err := migrateControls(path, "yandexmini_2", cfg)
		if err != nil {
			t.Fatal(err)
		}
		if out.TimingMode != mode || out.LEDs {
			t.Fatalf("migration changed timing or unrelated setting: %+v", out)
		}
	}
}
