package main

import (
	"fmt"
	"strconv"
	"time"
)

// Mix rendered frames, not raw cover colours. Calibration, visibility lifts
// and the black-cover threshold are nonlinear and must run before the fade.
type frameTransition struct {
	gen         int
	valid       bool
	start       time.Time
	from, shown []string
}

func (f *frameTransition) apply(target []string, gen int, now time.Time) []string {
	if len(target) == 0 {
		return target
	}
	if !f.valid || len(f.shown) != len(target) {
		f.gen, f.valid = gen, true
		f.start = now.Add(-artworkFade)
		f.shown = append([]string(nil), target...)
	}
	if gen != f.gen {
		f.gen, f.start = gen, now
		f.from = append([]string(nil), f.shown...)
	}
	p := float64(now.Sub(f.start)) / float64(artworkFade)
	if p >= 1 || len(f.from) != len(target) {
		f.shown = append(f.shown[:0], target...)
		return append([]string(nil), target...)
	}
	if p < 0 {
		p = 0
	}
	p = p * p * (3 - 2*p)
	out := make([]string, len(target))
	for i := range target {
		a, _ := strconv.ParseUint(f.from[i][1:], 16, 24)
		b, _ := strconv.ParseUint(target[i][1:], 16, 24)
		var rgb uint32
		for _, shift := range []uint{16, 8, 0} {
			x, y := float64(a>>shift&255), float64(b>>shift&255)
			rgb |= uint32(x+(y-x)*p+0.5) << shift
		}
		out[i] = fmt.Sprintf("#%06X", rgb)
	}
	f.shown = append(f.shown[:0], out...)
	return out
}

func (t *spectrumTap) frame(n int, level uint8) []string {
	frame := t.rawFrame(n, level)
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.spectrumFade.apply(frame, t.coverGen, time.Now())
}

func (t *spectrumTap) solidFrame(n int, level uint8) []string {
	frame := t.colorize(t.rawSolidFrame(n, level))
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.solidFade.apply(frame, t.coverGen, time.Now())
}
