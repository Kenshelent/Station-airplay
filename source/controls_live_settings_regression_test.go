package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A failed/slow native GET must not delay a genuinely new sender change,
// including while a station-to-phone handoff is awaiting acknowledgement.
type readFailStation struct {
	*mockStation
	reads int
}

func (s *readFailStation) GetVolume(context.Context) (float64, error) {
	s.reads++
	return 0, errors.New("native GET unavailable")
}

func TestLiveNewSliderChangeSkipsReadDuringHandoff(t *testing.T) {
	c, r, station := testControls()
	now := time.Now()
	station.v, r.s.DB = 35, percentDB(90)
	if err := c.tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if !c.remotePending {
		t.Fatal("expected pending initial handoff")
	}
	fault := &readFailStation{mockStation: station}
	c.station = fault
	r.s.DB = percentDB(60)
	if err := c.tick(context.Background(), now.Add(100*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if fault.reads != 0 || station.v != 60 || c.remotePending {
		t.Fatalf("new slider blocked: reads=%d native=%v pending=%t", fault.reads, station.v, c.remotePending)
	}
}

func TestLiveBrightnessSurvivesSteadyRepaint(t *testing.T) {
	c, _, station := testControls()
	tap := newSpectrumTap(t.TempDir())
	c.spectrum = tap
	c.cfg.LEDLevel = 200
	c.device = yamDevice{Model: "yandexmini_2"}
	tap.coverColor, tap.haveCoverColor = [3]float64{1, 1, 1}, true
	tap.solidFloor = 1
	cfg := c.cfg
	cfg.LEDLevel = 40
	if err := c.applyPanelSettings(cfg); err != nil {
		t.Fatal(err)
	}
	tap.mu.Lock()
	tap.gain = [3]float64{1, 1, 1}
	tap.mu.Unlock()
	now := time.Now()
	for i := 0; i < 10; i++ {
		if err := c.tick(context.Background(), now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if len(station.frames) == 0 {
		t.Fatal("no steady repaint")
	}
	for _, frame := range station.frames {
		for _, color := range frame {
			r, g, b := parseHex(t, color)
			if r > 55 || g > 55 || b > 55 {
				t.Fatalf("brightness reverted to startup 200: %s", color)
			}
		}
	}
	if got := currentLEDLevel(tap, 200); got != 40 {
		t.Fatalf("live level=%d", got)
	}
	if got := currentLEDLevel(nil, 16); got != 16 {
		t.Fatalf("legacy fallback=%d", got)
	}
}
