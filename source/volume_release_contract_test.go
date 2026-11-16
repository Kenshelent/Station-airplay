package main

import (
	"context"
	"math"
	"testing"
	"time"
)

// Pin the user-visible release contract independently of the implementation
// constants: changing both the constant and a constant-driven test must fail.
// 2.1.14 settles the sender grid at 6%: a 16-step AirPlay slider spans 100%,
// so 10% merged two neighbouring positions and 5% moved the merge to the next
// pair. 6% is closest to 100/16 and gives every notch its own step.
func TestReleaseVolumeContract(t *testing.T) {
	if volumeStepPercent != 6 || volumeCurveExponent != 0.8 {
		t.Fatal("release requires the 6-percent sender grid and 0.8 curve")
	}
	seen := make(map[int]bool)
	previous := -1
	for i := 0; i <= 3000; i++ {
		db := -30 + float64(i)/100
		got := dbPercent(db)
		if got < previous || got < 0 || got > 100 || (got != 100 && got%6 != 0) {
			t.Fatalf("invalid sender mapping at %g dB: %d after %d", db, got, previous)
		}
		seen[got] = true
		previous = got
	}
	for percent := 0; percent <= 96; percent += 6 {
		if !seen[percent] || dbPercent(percentDB(percent)) != percent {
			t.Fatalf("unreachable or unstable sender step %d", percent)
		}
	}
	if !seen[100] || dbPercent(percentDB(100)) != 100 {
		t.Fatal("the top step is unreachable")
	}
	for percent := 5; percent <= 95; percent += 5 {
		got, err := nativePercent(float64(percent))
		if err != nil || got != percent {
			t.Fatalf("native %d must not snap to sender grid: %d, %v", percent, got, err)
		}
	}
}

func TestReleaseSenderCurveReachesStation(t *testing.T) {
	for _, tc := range []struct {
		db   float64
		want int
	}{
		{-144, 0}, {-30, 0}, {-25, 24}, {-20, 42},
		{-15, 60}, {-10, 72}, {-5, 84}, {0, 100},
	} {
		c, receiver, station := testControls()
		ctx, now := context.Background(), time.Now()
		if err := c.tick(ctx, now); err != nil {
			t.Fatal(err)
		}
		receiver.s.DB = tc.db
		if err := c.tick(ctx, now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		// The station's own volume grid is 5%, so the written step is snapped:
		// 24 becomes 25, 42 becomes 40. What matters is that the request and
		// the readback both move with the sender, not that 6% survives as-is.
		snapped, err := nativePercent(float64(tc.want))
		if err != nil {
			t.Fatal(err)
		}
		if len(station.sets) != 1 || station.sets[0] != tc.want || station.v != float64(snapped) {
			t.Fatalf("sender %g dB: writes %v, station %g; want write %d, station %d", tc.db, station.sets, station.v, tc.want, snapped)
		}
	}
}

func TestReleaseInvalidSenderVolumeNeverWrites(t *testing.T) {
	for _, db := range []float64{-31, 1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		c, receiver, station := testControls()
		ctx, now := context.Background(), time.Now()
		if err := c.tick(ctx, now); err != nil {
			t.Fatal(err)
		}
		receiver.s.DB = db
		if err := c.tick(ctx, now.Add(time.Second)); err == nil {
			t.Fatalf("accepted invalid sender volume %g", db)
		}
		if len(station.sets) != 0 || station.v != 70 {
			t.Fatalf("invalid sender changed volume: %v, %g", station.sets, station.v)
		}
	}
}

func TestReleaseNativeHandoffDoesNotRaiseFivePercentLevel(t *testing.T) {
	c, receiver, station := testControls()
	station.v = 25
	receiver.s.DB = -10
	ctx, now := context.Background(), time.Now()
	if err := c.tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(station.sets) != 0 || station.v != 25 || len(receiver.volumes) != 1 {
		t.Fatalf("handoff rewrote native volume: %v, %g, %v", station.sets, station.v, receiver.volumes)
	}
	want := 30*math.Pow(0.25, 1.25) - 30
	if math.Abs(receiver.volumes[0]-want) > 1e-9 {
		t.Fatalf("feedback %g, want inverse curve %g", receiver.volumes[0], want)
	}
	// The sender acknowledges our own feedback; this must not snap 25 to 30.
	receiver.s.DB = receiver.volumes[0]
	for tick := 1; tick <= 4; tick++ {
		if err := c.tick(ctx, now.Add(time.Duration(tick)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if len(station.sets) != 0 || station.v != 25 {
		t.Fatalf("feedback changed station: %v, %g", station.sets, station.v)
	}
}
