package main

import (
	"fmt"
	"math"
	"strconv"
)

// spectrumPoints are the editable nodes of the artwork colour table. A node
// stores the colour the LEDs should actually show for that nominal artwork
// colour, so tuning orange does not shift red and yellow with it. Black is
// configured separately: a black cover has a per-model fallback (off, stock
// blue-grey or a dim glow) that is not part of this table.
type spectrumPoints struct {
	Grey    string `json:"grey,omitempty"`
	Red     string `json:"red,omitempty"`
	Green   string `json:"green,omitempty"`
	Blue    string `json:"blue,omitempty"`
	Yellow  string `json:"yellow,omitempty"`
	Cyan    string `json:"cyan,omitempty"`
	Magenta string `json:"magenta,omitempty"`
	Orange  string `json:"orange,omitempty"`
	White   string `json:"white,omitempty"`
}

// colorLUT is a 3x3x3 table over input RGB, with nodes at 0, 0.5 and 1 on
// each axis. Only the nine named points are edited; the remaining nodes stay
// at identity, so an untouched table is a no-op.
type colorLUT [27][3]float64

type spectrumNode struct {
	name string
	r, g int
	b    int
}

var spectrumNodes = []spectrumNode{
	{"grey", 1, 1, 1},
	{"red", 2, 0, 0},
	{"green", 0, 2, 0},
	{"blue", 0, 0, 2},
	{"yellow", 2, 2, 0},
	{"cyan", 0, 2, 2},
	{"magenta", 2, 0, 2},
	{"orange", 2, 1, 0},
	{"white", 2, 2, 2},
}

func colorLUTIndex(r, g, b int) int { return (r*3+g)*3 + b }

func identityColorLUT() *colorLUT {
	var l colorLUT
	for r := 0; r < 3; r++ {
		for g := 0; g < 3; g++ {
			for b := 0; b < 3; b++ {
				l[colorLUTIndex(r, g, b)] = [3]float64{float64(r) / 2, float64(g) / 2, float64(b) / 2}
			}
		}
	}
	return &l
}

func defaultSpectrumPoints() spectrumPoints {
	return spectrumPoints{
		Grey:    "#808080",
		Red:     "#FF0000",
		Green:   "#00FF00",
		Blue:    "#0000FF",
		Yellow:  "#FFFF00",
		Cyan:    "#00FFFF",
		Magenta: "#FF00FF",
		Orange:  "#FF8000",
		White:   "#FFFFFF",
	}
}

func (p *spectrumPoints) validate() error {
	if p == nil {
		return nil
	}
	for _, node := range spectrumNodes {
		value := p.value(node.name)
		if value != "" && !previewColor.MatchString(value) {
			return fmt.Errorf("spectrum_points.%s must be #RRGGBB", node.name)
		}
	}
	return nil
}

func (p *spectrumPoints) value(name string) string {
	if p == nil {
		return ""
	}
	switch name {
	case "grey":
		return p.Grey
	case "red":
		return p.Red
	case "green":
		return p.Green
	case "blue":
		return p.Blue
	case "yellow":
		return p.Yellow
	case "cyan":
		return p.Cyan
	case "magenta":
		return p.Magenta
	case "orange":
		return p.Orange
	case "white":
		return p.White
	}
	return ""
}

// lut returns nil while every point is unset, so the renderer keeps its
// previous fast path. A partial table starts from identity.
func (p *spectrumPoints) lut() *colorLUT {
	if p == nil {
		return nil
	}
	var set bool
	l := identityColorLUT()
	for _, node := range spectrumNodes {
		value := p.value(node.name)
		if value == "" {
			continue
		}
		v, err := strconv.ParseUint(value[1:], 16, 24)
		if err != nil {
			continue
		}
		set = true
		l[colorLUTIndex(node.r, node.g, node.b)] = [3]float64{
			float64(v>>16&255) / 255,
			float64(v>>8&255) / 255,
			float64(v&255) / 255,
		}
	}
	if !set {
		return nil
	}
	return l
}

// mapColor interpolates the table trilinearly. Inputs outside 0..1 are
// clamped, matching the renderer's own clamping.
func (l *colorLUT) mapColor(c [3]float64) [3]float64 {
	if l == nil {
		return c
	}
	x := math.Min(1, math.Max(0, c[0])) * 2
	y := math.Min(1, math.Max(0, c[1])) * 2
	z := math.Min(1, math.Max(0, c[2])) * 2
	i0, j0, k0 := int(x), int(y), int(z)
	if i0 > 1 {
		i0 = 1
	}
	if j0 > 1 {
		j0 = 1
	}
	if k0 > 1 {
		k0 = 1
	}
	fx, fy, fz := x-float64(i0), y-float64(j0), z-float64(k0)
	var out [3]float64
	for di := 0; di <= 1; di++ {
		wx := fx
		if di == 0 {
			wx = 1 - fx
		}
		for dj := 0; dj <= 1; dj++ {
			wy := fy
			if dj == 0 {
				wy = 1 - fy
			}
			for dk := 0; dk <= 1; dk++ {
				wz := fz
				if dk == 0 {
					wz = 1 - fz
				}
				w := wx * wy * wz
				if w == 0 {
					continue
				}
				node := l[colorLUTIndex(i0+di, j0+dj, k0+dk)]
				for ch := 0; ch < 3; ch++ {
					out[ch] += w * node[ch]
				}
			}
		}
	}
	for ch := 0; ch < 3; ch++ {
		out[ch] = math.Min(1, math.Max(0, out[ch]))
	}
	return out
}
