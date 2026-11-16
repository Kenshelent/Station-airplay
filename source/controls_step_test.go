package main

import (
	"math"
	"testing"
)

func TestVolumeStepAndLoudnessCurve(t *testing.T) {
	if got := dbPercent(-144); got != 0 {
		t.Fatalf("mute = %d", got)
	}
	if got := dbPercent(-30); got != 0 {
		t.Fatalf("floor = %d", got)
	}
	if got := dbPercent(0); got != 100 {
		t.Fatalf("top = %d", got)
	}
	if got := dbPercent(-10); got != 72 {
		t.Fatalf("-10 dB = %d, want 72", got)
	}
	if got := dbPercent(-20); got != 42 {
		t.Fatalf("-20 dB = %d, want 42", got)
	}
	previous := -1
	for db := -30.0; db <= 0; db += 0.5 {
		got := dbPercent(db)
		if got < previous {
			t.Fatalf("not monotonic at %.1f: %d after %d", db, got, previous)
		}
		if got != 100 && got%volumeStepPercent != 0 {
			t.Fatalf("%.1f dB = %d, not a %d%% step", db, got, volumeStepPercent)
		}
		previous = got
	}
	for percent := 0; percent <= 100; percent += volumeStepPercent {
		back := dbPercent(percentDB(percent))
		if back != percent {
			t.Fatalf("round trip %d -> %.3f dB -> %d", percent, percentDB(percent), back)
		}
	}
	if !math.IsInf(percentDB(0), 0) && percentDB(0) != -144 {
		t.Fatalf("zero percent = %.2f", percentDB(0))
	}
}

// An AirPlay sender exposes about sixteen slider positions over -30..0 dB.
// Each one must land on its own station step; with the old 10% grid two
// neighbouring positions collapsed and moving the slider by one notch did
// nothing on Mini.
func TestEverySenderPositionReachesItsOwnStep(t *testing.T) {
	seen := map[int]float64{}
	for i := 1; i <= 15; i++ {
		db := -30 + 2*float64(i)
		got := dbPercent(db)
		if previous, ok := seen[got]; ok {
			t.Fatalf("%.0f dB and %.0f dB both map to %d%%", previous, db, got)
		}
		seen[got] = db
	}
	if len(seen) != 15 {
		t.Fatalf("only %d distinct steps out of 15", len(seen))
	}
	if got := dbPercent(-30); got != 0 {
		t.Fatalf("bottom = %d", got)
	}
	if got := dbPercent(0); got != 100 {
		t.Fatalf("top = %d", got)
	}
}

func TestNativeVolumeObservationKeepsFivePercentSteps(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want int
	}{
		{0, 0}, {2, 0}, {4, 5}, {5, 5}, {7, 5}, {8, 10}, {25, 25}, {74, 75}, {100, 100},
	} {
		got, err := nativePercent(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("nativePercent(%v) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}
