package main

import "testing"

func TestPanelDefaultsUseRendererProfiles(t *testing.T) {
	for _, model := range []string{"yandexmini", "yandexmini_2", "yandexmini3", "bergamot", "yandexmicro", "plum", "yandexplum", "yandexmidi", "mango", "yandexstation_2", "unlisted-board"} {
		d := yamDevice{Model: model}
		cfg := panelDefaults(d)
		gain, _, _, _ := spectrumDefaults(d, controlsConfig{})
		floor, _, saturation := spectrumComfort(d)
		if cfg.SpectrumGainR != gain[0] || cfg.SpectrumGainG != gain[1] || cfg.SpectrumGainB != gain[2] {
			t.Fatal("duplicated or guessed defaults", model, cfg)
		}
		if cfg.SpectrumBlackFloor == nil || *cfg.SpectrumBlackFloor != floor || cfg.SpectrumSaturation != saturation {
			t.Fatal("comfort differs", model, cfg)
		}
		if cfg.TimingMode != timingFastAudio {
			t.Fatal("panel lost -1.7 default", model)
		}
		if model == "yandexmini" && (cfg.Spectrum == nil || *cfg.Spectrum) {
			t.Fatal("logo defaults must be solid")
		}
	}
}
