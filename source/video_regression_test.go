package main

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type outputTestStation struct {
	*mockStation
	writeErr error
}

func (s *outputTestStation) Colors(ctx context.Context, colors []string) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	return s.mockStation.Colors(ctx, colors)
}

func TestLEDOutputOnlyAcknowledgedWrites(t *testing.T) {
	station := &outputTestStation{mockStation: &mockStation{}}
	c := &sharedController{station: station}
	colors := []string{"#AA00BB", "#112233"}
	station.writeErr = errors.New("LED backend busy")
	if err := c.writeLEDColors(context.Background(), colors); err == nil {
		t.Fatal("rejected write was accepted")
	}
	s := c.output.snapshot(time.Now())
	if s.Sequence != 0 || len(s.Colors) != 0 || s.State != "error" {
		t.Fatalf("rejected output published: %+v", s)
	}
	station.writeErr = nil
	if err := c.writeLEDColors(context.Background(), colors); err != nil {
		t.Fatal(err)
	}
	colors[0] = "#FFFFFF"
	s = c.output.snapshot(time.Now())
	if s.Sequence != 1 || s.Colors[0] != "#AA00BB" || s.State != "sent" || s.Error != "" {
		t.Fatalf("wrong acknowledged output: %+v", s)
	}
	s.Colors[0] = "#000000"
	// Controller gates may change without an LED write. They must not blank
	// the reported last successful output.
	c.ledPlay.Store(false)
	c.ledSolid.Store(false)
	station.writeErr = errors.New("offline")
	_ = c.writeLEDColors(context.Background(), []string{"#000000"})
	s = c.output.snapshot(time.Now())
	if s.Sequence != 1 || s.Colors[0] != "#AA00BB" || s.State != "error" {
		t.Fatalf("failed write replaced acknowledged frame: %+v", s)
	}
	station.writeErr = nil
	if err := c.writeLEDOff(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s = c.output.snapshot(time.Now()); s.Sequence != 2 || s.State != "released" {
		t.Fatalf("handover not reported: %+v", s)
	}
}

func TestLEDOutputAgeAndConcurrentSnapshots(t *testing.T) {
	var output ledOutput
	now := time.Now()
	output.record([]string{"#123456"}, [4]float64{.1, .2, .3, .4}, false, nil, now)
	s := output.snapshot(now.Add(4 * time.Second))
	if s.AgeMS != 4000 || s.SentAtMS != now.UnixMilli() || s.Sequence != 1 {
		t.Fatalf("wrong snapshot timing: %+v", s)
	}
	// Reading status never creates frames or resets their age.
	if output.snapshot(now.Add(5*time.Second)).AgeMS != 5000 {
		t.Fatal("status read refreshed stale data")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				output.record([]string{"#123456"}, [4]float64{}, false, nil, now)
				_ = output.snapshot(now)
			}
		}()
	}
	wg.Wait()
}

func TestAnalyserCaptureContract(t *testing.T) {
	for _, rate := range []int{48000, 96000} {
		conf := uiAsoundSpectrumAtRate("softvol", audioFIFO, spectrumFIFO, rate, "linear")
		// The format/rate constraint is above both file PCMs, while a plug
		// below the taps adapts S16_LE stereo to the firmware's native PCM.
		for _, want := range []string{
			`pcm.spk_output`, `slave.pcm "softvol"`, `slave.pcm "spk_output"`,
			`format "S16_LE"`, `channels 2`, `defaults.pcm.rate_converter "linear"`,
		} {
			if !strings.Contains(conf, want) {
				t.Fatalf("rate %d: missing %q:\n%s", rate, want, conf)
			}
		}
		if strings.Count(conf, "type file") != 2 {
			t.Fatal("clock and spectrum must keep independent file taps")
		}
	}
}

func TestPCMChunkBoundariesDoNotChangeAnalysis(t *testing.T) {
	rate := 48000
	data := make([]byte, rate/2*4)
	for i := 0; i < len(data)/4; i++ {
		v := int16(14000 * math.Sin(2*math.Pi*330*float64(i)/float64(rate)))
		binary.LittleEndian.PutUint16(data[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[i*4+2:], uint16(v))
	}
	now := time.Now()
	whole := &airplayState{sampleRate: rate}
	whole.audio(data, now)
	for _, chunk := range []int{1, 3, 7, 97, 4097} {
		split := &airplayState{sampleRate: rate}
		for offset := 0; offset < len(data); offset += chunk {
			end := offset + chunk
			if end > len(data) {
				end = len(data)
			}
			split.audio(data[offset:end], now)
		}
		if !reflect.DeepEqual(whole.history, split.history) || whole.pcmFrames != split.pcmFrames {
			t.Fatalf("analysis depends on FIFO read boundaries (chunk %d): %v vs %v", chunk, whole.history, split.history)
		}
	}
}
