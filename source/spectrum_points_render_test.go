package main

import "testing"

func TestSolidFrameAppliesColourPoints(t *testing.T) {
	tap := newSpectrumTap("")
	tap.coverColor = [3]float64{1, .5, 0}
	tap.haveCoverColor = true
	tap.lut = (&spectrumPoints{Orange: "#400000"}).lut()
	frame := tap.rawSolidFrame(1, 200)
	if len(frame) != 1 || frame[0] != "#400000" {
		t.Fatalf("calibrated orange was not used: %v", frame)
	}
}

func TestSolidFrameLeavesBlackPointToBlackControls(t *testing.T) {
	tap := newSpectrumTap("")
	tap.coverColor = [3]float64{0, 0, 0}
	tap.haveCoverColor = true
	tap.neutralBlack = true
	tap.blackFloor = .07
	tap.blackColor = &[3]float64{1, 0, 0}
	tap.lut = (&spectrumPoints{White: "#000000"}).lut()
	frame := tap.rawSolidFrame(1, 200)
	want := "#230000"
	if len(frame) != 1 || frame[0] != want {
		t.Fatalf("black cover must keep its own colour, got %v want %s", frame, want)
	}
}
