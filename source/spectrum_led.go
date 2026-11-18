package main

// runSpectrumLEDs paints the real PCM bands at 10 Hz. The control tick still
// owns the gates (Alice idle, speech, volume overlay, pause); it publishes a
// single atomic flag and never blocks on the LED request itself.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"time"
)

func spectrumLEDLevel(cfg controlsConfig) uint8 {
	level := cfg.LEDLevel
	if level == 0 {
		level = 200
	}
	return level
}

// animationInterval is the LED frame period. 50 ms (20 fps) makes colour
// transitions and fast transients look continuous, while staying well inside
// what the station's LED API accepts.
const animationInterval = 50 * time.Millisecond

// blendFrames eases from the shown frame towards the next one, so a cover
// change or a band jump does not flash.
func blendFrames(shown, next []string, factor float64) []string {
	if len(shown) != len(next) {
		return append([]string(nil), next...)
	}
	out := make([]string, len(next))
	for i := range next {
		ar, ag, ab, ok := parseLedColor(shown[i])
		br, bg, bb, ok2 := parseLedColor(next[i])
		if !ok || !ok2 {
			out[i] = next[i]
			continue
		}
		blend := func(a, b int) int {
			return int(math.Round(float64(a) + float64(b-a)*factor))
		}
		out[i] = fmt.Sprintf("#%02X%02X%02X", blend(ar, br), blend(ag, bg), blend(ab, bb))
	}
	return out
}

func parseLedColor(value string) (int, int, int, bool) {
	if len(value) != 7 || value[0] != '#' {
		return 0, 0, 0, false
	}
	var rgb [3]int
	for i := 0; i < 3; i++ {
		hi := hexValue(value[1+i*2])
		lo := hexValue(value[2+i*2])
		if hi < 0 || lo < 0 {
			return 0, 0, 0, false
		}
		rgb[i] = hi*16 + lo
	}
	return rgb[0], rgb[1], rgb[2], true
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return -1
}

func runSpectrumLEDs(ctx context.Context, c *sharedController, tap *spectrumTap, _ uint8) {
	ticker := time.NewTicker(animationInterval)
	defer ticker.Stop()
	var lastErr time.Time
	var shown []string
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.preview.active(time.Now()) {
				continue
			}
			// Read the brightness every tick: the panel changes it live.
			level := uint8(tap.ledLevel.Load())
			if level == 0 {
				level = 200
			}
			if !c.ledPlay.Load() && !c.ledSolid.Load() {
				continue
			}
			colors := tap.frame(c.ledCount, level)
			if c.ledSolid.Load() {
				colors = tap.solidFrame(c.ledCount, level)
			}
			if len(colors) == 0 {
				continue
			}
			c.ledMu.Lock()
			if c.preview.active(time.Now()) {
				c.ledMu.Unlock()
				continue
			}
			if !c.ledPlay.Load() && !c.ledSolid.Load() {
				c.ledMu.Unlock()
				continue
			}
			if len(shown) != len(colors) {
				shown = append([]string(nil), colors...)
			} else {
				shown = blendFrames(shown, colors, .45)
			}
			err := c.writeLEDColors(ctx, shown)
			c.ledMu.Unlock()
			if err != nil {
				if errors.Is(err, errFramebufferBusy) {
					// The station has not released its LED thread yet. Keep the
					// animator alive; the next tick retries the lease.
					continue
				}
				c.ledFailed.Store(true)
				if time.Since(lastErr) > 10*time.Second {
					fmt.Fprintf(os.Stderr, "airplay spectrum: LED frame: %v\n", err)
					lastErr = time.Now()
				}
				return
			}
		}
	}
}
