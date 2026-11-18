package main

import (
	"context"
	"time"
)

func originalMini(d yamDevice) bool {
	return d.Model == "yandexmini" || d.Model == "YandexStationMini"
}

func controlsTickInterval(d yamDevice) time.Duration {
	if originalMini(d) || d.Model == "yandexmini_2" {
		return 100 * time.Millisecond
	}
	return 500 * time.Millisecond
}

// Restore the Mini contributor's fast write path only on that board and only
// when its Quasar volume-manager is offline. Luna still reads the logical
// volume. Physical readback/slider agreement is a qualification gate, not
// something this candidate assumes for the other models.
type miniMixerWriteVolume struct {
	reader volumeControl
	mixer  *alsaControlVolume
}

func (v *miniMixerWriteVolume) Backend() string {
	return "ALSA Master writes, Luna /api/volume reads (original Mini only)"
}

func (v *miniMixerWriteVolume) GetVolume(ctx context.Context) (float64, error) {
	return v.reader.GetVolume(ctx)
}

func (v *miniMixerWriteVolume) SetVolume(ctx context.Context, pct int) (float64, error) {
	return v.mixer.SetVolume(ctx, pct)
}

func (v *miniMixerWriteVolume) CheckWrite(ctx context.Context) error {
	return v.reader.CheckWrite(ctx)
}
