package main

import (
	"context"
	"sync"
	"time"
)

// ledOutput describes successful host API writes, not the physical LED state:
// Alice, a volume overlay or another host client may subsequently take over.
type ledOutput struct {
	mu       sync.Mutex
	sequence uint64
	at       time.Time
	colors   []string
	bands    [4]float64
	released bool
	lastErr  string
}

type ledOutputSnapshot struct {
	Sequence uint64     `json:"sequence"`
	SentAtMS int64      `json:"sent_at_ms"`
	AgeMS    int64      `json:"age_ms"`
	State    string     `json:"state"`
	Colors   []string   `json:"colors"`
	Bands    [4]float64 `json:"bands"`
	Error    string     `json:"error,omitempty"`
}

func (s *ledOutput) record(colors []string, bands [4]float64, released bool, err error, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastErr = err.Error()
		return // A rejected write must not change the last acknowledged frame.
	}
	s.sequence++
	s.at, s.bands, s.released, s.lastErr = now, bands, released, ""
	s.colors = append([]string(nil), colors...)
}

func (s *ledOutput) snapshot(now time.Time) ledOutputSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := ledOutputSnapshot{
		Sequence: s.sequence, State: "unavailable", AgeMS: -1,
		Colors: append([]string(nil), s.colors...), Bands: s.bands, Error: s.lastErr,
	}
	if !s.at.IsZero() {
		out.SentAtMS, out.AgeMS = s.at.UnixMilli(), now.Sub(s.at).Milliseconds()
		if out.AgeMS < 0 {
			out.AgeMS = 0
		}
		out.State = "sent"
		if s.released {
			out.State = "released"
		}
	}
	if s.lastErr != "" {
		out.State = "error"
	}
	return out
}

// Callers retain their existing ledMu ownership. Record all RGB paths,
// including solid artwork and preview, without changing station interfaces.
func (c *sharedController) writeLEDColors(ctx context.Context, colors []string) error {
	var bands [4]float64
	if c.spectrum != nil {
		c.spectrum.mu.Lock()
		bands = c.spectrum.state.history
		c.spectrum.mu.Unlock()
	}
	err := c.station.Colors(ctx, colors)
	c.output.record(colors, bands, false, err, time.Now())
	return err
}

func (c *sharedController) writeLEDOff(ctx context.Context) error {
	err := c.station.Off(ctx)
	c.output.record(nil, [4]float64{}, true, err, time.Now())
	return err
}
