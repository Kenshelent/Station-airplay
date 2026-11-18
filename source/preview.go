package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

const previewTTL = 4 * time.Second

var previewColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// A preview is an expiring lease, not a one-shot write competing with AirPlay.
// The control tick supplies a short safety gate; a stalled controller fails
// closed. No maind process is stopped and Alice/volume retain priority.
type ledPreview struct {
	mu                 sync.Mutex
	id                 string
	colors             []string
	expires, safeUntil time.Time
	reason, lastError  string
	drawn              bool
	volumeKnown        bool
	volume             float64
	volumeQuiet        time.Time
}

func (p *ledPreview) active(now time.Time) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.drawn || p.id != "" && now.Before(p.expires)
}

func (p *ledPreview) gate(now time.Time, allowed bool, reason string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reason = reason
	p.safeUntil = time.Time{}
	if now.Before(p.volumeQuiet) {
		allowed, p.reason = false, "volume overlay"
	}
	if allowed {
		p.safeUntil = now.Add(1500 * time.Millisecond)
	}
}

func (p *ledPreview) observeVolume(value float64, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.volumeKnown && value != p.volume {
		p.volumeQuiet = now.Add(2 * time.Second)
	}
	p.volumeKnown, p.volume = true, value
}

func (p *ledPreview) acquire(id string, colors []string, count int, now time.Time) (string, error) {
	if count <= 0 || len(colors) != count {
		return "", fmt.Errorf("expected %d LED colours", count)
	}
	for _, color := range colors {
		if !previewColor.MatchString(color) {
			return "", errors.New("colour must be #RRGGBB")
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.id != "" && now.Before(p.expires) && id != p.id {
		return "", errors.New("preview is owned by another session")
	}
	if id == "" || id != p.id {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", err
		}
		id = hex.EncodeToString(token[:])
		p.safeUntil = time.Time{} // a new owner waits for a fresh controller gate
	}
	p.id, p.colors, p.expires = id, append([]string(nil), colors...), now.Add(previewTTL)
	return id, nil
}

func (p *ledPreview) stop(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.id != "" && id != p.id {
		return errors.New("preview session does not match")
	}
	p.id, p.colors, p.expires = "", nil, time.Time{}
	return nil
}

func (p *ledPreview) status(now time.Time) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	active := p.id != "" && now.Before(p.expires)
	reason := p.reason
	if active && !now.Before(p.safeUntil) && reason == "" {
		reason = "control gate expired"
	}
	return map[string]any{"active": active, "allowed": active && now.Before(p.safeUntil),
		"expires_at": p.expires, "suppressed": reason, "last_error": p.lastError}
}

func (p *ledPreview) paint(ctx context.Context, c *sharedController, now time.Time) {
	// Serialize with spectrum/control output. Hold the lease lock through the
	// write so stop/acquire cannot acknowledge a frame still being sent.
	c.ledMu.Lock()
	defer c.ledMu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	allowed := p.id != "" && now.Before(p.expires) && now.Before(p.safeUntil)
	var err error
	if allowed {
		colors := append([]string(nil), p.colors...)
		if c.spectrum != nil {
			colors = c.spectrum.colorize(colors)
		}
		err = c.writeLEDColors(ctx, colors)
		p.drawn = err == nil
	} else if p.drawn {
		if releaser, ok := c.station.(ringReleaser); ok {
			err = releaser.Release(ctx)
			c.output.record(nil, [4]float64{}, true, err, time.Now())
		} else if p.reason == "" && now.Before(p.safeUntil) {
			err = c.writeLEDOff(ctx)
		}
		p.drawn = false
	}
	if err != nil {
		p.lastError = err.Error()
	} else {
		p.lastError = ""
	}
}

func runPreview(ctx context.Context, c *sharedController) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		c.preview.gate(time.Now(), false, "service stopped")
		stop, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c.preview.paint(stop, c, time.Now())
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			c.preview.paint(ctx, c, now)
		}
	}
}
