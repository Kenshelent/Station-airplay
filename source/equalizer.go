package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

var eqFrequencies = [5]float64{60, 250, 1000, 4000, 12000}

// Experimental, AirPlay PCM only. Stock music and Alice are never filtered.
type equalizerConfig struct {
	Enabled  bool       `json:"enabled"`
	PreampDB float64    `json:"preamp_db"`
	BandsDB  [5]float64 `json:"bands_db"`
}

func (e *equalizerConfig) UnmarshalJSON(data []byte) error {
	var value struct {
		Enabled  bool      `json:"enabled"`
		PreampDB float64   `json:"preamp_db"`
		BandsDB  []float64 `json:"bands_db"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if value.BandsDB != nil && len(value.BandsDB) != 5 {
		return errors.New("equalizer bands_db must have exactly five values")
	}
	e.Enabled, e.PreampDB = value.Enabled, value.PreampDB
	for i := range e.BandsDB {
		e.BandsDB[i] = 0
	}
	copy(e.BandsDB[:], value.BandsDB)
	return e.validate()
}

func (e *equalizerConfig) validate() error {
	if e == nil {
		return nil
	}
	if math.IsNaN(e.PreampDB) || math.IsInf(e.PreampDB, 0) || e.PreampDB < -24 || e.PreampDB > 0 {
		return errors.New("equalizer preamp_db must be -24..0")
	}
	for _, g := range e.BandsDB {
		if math.IsNaN(g) || math.IsInf(g, 0) || g < -12 || g > 12 {
			return errors.New("equalizer bands_db must be -12..12")
		}
	}
	return nil
}

type biquad struct {
	b0, b1, b2, a1, a2 float64
	z1, z2             [2]float64
}

func peakingEQ(rate, hz, gain float64) biquad {
	a := math.Pow(10, gain/40)
	w := 2 * math.Pi * hz / rate
	alpha := math.Sin(w) / 2
	a0 := 1 + alpha/a
	return biquad{b0: (1 + alpha*a) / a0, b1: -2 * math.Cos(w) / a0, b2: (1 - alpha*a) / a0, a1: -2 * math.Cos(w) / a0, a2: (1 - alpha/a) / a0}
}
func (b *biquad) sample(x float64, channel int) float64 {
	y := b.b0*x + b.z1[channel]
	b.z1[channel] = b.b1*x - b.a1*y + b.z2[channel]
	b.z2[channel] = b.b2*x - b.a2*y
	return y
}
func processEqualizer(r io.Reader, w io.Writer, cfg equalizerConfig, rate int) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if rate != 44100 && rate != 48000 {
		return errors.New("equalizer rate must be 44100 or 48000")
	}
	if !cfg.Enabled {
		_, err := io.Copy(w, r)
		return err
	}
	filters := make([]biquad, 5)
	headroom := cfg.PreampDB
	for i, g := range cfg.BandsDB {
		filters[i] = peakingEQ(float64(rate), eqFrequencies[i], g)
		if g > 0 {
			headroom -= g
		}
	}
	gain := math.Pow(10, headroom/20)
	buf := make([]byte, 4096)
	for {
		n, err := io.ReadFull(r, buf)
		if n%4 != 0 {
			return errors.New("truncated stereo S16_LE frame")
		}
		for i := 0; i < n; i += 2 {
			x := float64(int16(binary.LittleEndian.Uint16(buf[i:i+2]))) * gain
			for j := range filters {
				x = filters[j].sample(x, (i/2)%2)
			}
			x = math.Max(-32768, math.Min(32767, math.Round(x)))
			binary.LittleEndian.PutUint16(buf[i:i+2], uint16(int16(x)))
		}
		if n > 0 {
			if _, e := w.Write(buf[:n]); e != nil {
				return e
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
func eqWorker(ctx context.Context, path, pcm string, rate int) error {
	cfg, err := loadControls(path)
	if err != nil {
		return err
	}
	if cfg == nil || cfg.Equalizer == nil {
		return errors.New("equalizer configuration missing")
	}
	aplay, err := exec.LookPath("aplay")
	if err != nil {
		return errors.New("experimental AirPlay EQ requires firmware aplay")
	}
	cmd := exec.CommandContext(ctx, aplay, "-q", "-D", pcm, "-t", "raw", "-f", "S16_LE", "-c", "2", "-r", strconv.Itoa(rate))
	cmd.Env = runtimeEnv(os.Environ(), "", "")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	err = processEqualizer(os.Stdin, in, *cfg.Equalizer, rate)
	_ = in.Close()
	if err != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if err != nil {
		return err
	}
	return waitErr
}
func quoteShell(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func eqAsound(exe, config, pcm string, rate int) string {
	command := "|exec " + quoteShell(exe) + " --eq-worker --controls-config " + quoteShell(config) + " --eq-pcm " + quoteShell(pcm) + " --eq-rate " + strconv.Itoa(rate)
	return fmt.Sprintf("pcm.spk_equalizer {\n type file\n slave.pcm \"null\"\n file %q\n format \"raw\"\n}\n", command)
}
