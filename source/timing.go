package main

import "errors"

const (
	timingSynchronized = "synchronized"
	timingFastAudio    = "fast_audio"
)

func (c *controlsConfig) validateTiming() error {
	if c.TimingMode != "" && c.TimingMode != timingSynchronized && c.TimingMode != timingFastAudio {
		return errors.New("timing_mode must be synchronized or fast_audio")
	}
	return nil
}

// profileWithTiming copies an explicitly configured mode onto this invocation.
// A nil config preserves an already selected mode when controls fall back to
// audio-only. Old files without the key use the synchronized default without a
// rewrite, so an upgrade never needs to reset the owner's other settings.
func profileWithTiming(p stationProfile, c *controlsConfig) stationProfile {
	if c != nil {
		p.timingMode = c.TimingMode
	}
	if p.timingMode == "" {
		p.timingMode = timingSynchronized
	}
	return p
}

func (p stationProfile) latencyOffset() float64 {
	if p.timingMode == timingFastAudio {
		return -1.7
	}
	return 0
}
