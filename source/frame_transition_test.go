package main

import (
	"testing"
	"time"
)

func TestRenderedFrameTransition(t *testing.T) {
	var f frameTransition
	now := time.Unix(1000, 0)
	black := []string{"#0C0218"} // rendered violet, not raw #000000
	white := []string{"#FFFFFF"}
	if got := f.apply(black, 1, now); got[0] != black[0] {
		t.Fatal(got)
	}
	if got := f.apply(white, 2, now); got[0] != black[0] {
		t.Fatal("hard cut", got)
	}
	got := f.apply(white, 2, now.Add(artworkFade/2))
	if got[0] != "#86818C" {
		t.Fatal("incorrect midpoint", got)
	}
	if got := f.apply(white, 2, now.Add(artworkFade)); got[0] != white[0] {
		t.Fatal(got)
	}
}

func TestRenderedFrameRapidSkipStartsFromCurrentOutput(t *testing.T) {
	var f frameTransition
	now := time.Unix(1000, 0)
	f.apply([]string{"#FF0000"}, 1, now)
	f.apply([]string{"#0000FF"}, 2, now)
	shown := f.apply([]string{"#0000FF"}, 2, now.Add(artworkFade/2))
	got := f.apply([]string{"#00FF00"}, 3, now.Add(artworkFade/2))
	if got[0] != shown[0] {
		t.Fatalf("skip jumped: %v -> %v", shown, got)
	}
	got[0] = "#FFFFFF"
	if f.shown[0] == got[0] {
		t.Fatal("output aliases internal state")
	}
}
