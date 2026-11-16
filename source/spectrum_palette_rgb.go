package main

import (
	"bytes"
	"image"
	"io"
	"math"
	"sort"

	// Apple sends album art as TIFF in some AirPlay streams, and shairport
	// caches it with a .tif name. Without this decoder those covers never
	// produced a colour and the solid indication fell back to its placeholder.
	_ "golang.org/x/image/tiff"
)

// coverRGB keeps saturation and brightness instead of expanding every tint
// (including cream paper and skin) into a fully saturated HSV ring.
func coverRGB(r io.Reader, size int64) ([3]float64, bool) {
	rgb, _, ok := coverPaletteRGB(r, size, 0)
	return rgb, ok
}

// coverPaletteRGB decodes the artwork once and returns both the dominant
// colour and up to want distinct colours, so a multi-zone indication can follow
// the cover's palette instead of one flat tint.
func coverPaletteRGB(r io.Reader, size int64, want int) ([3]float64, [][3]float64, bool) {
	var zero [3]float64
	if size <= 0 || size > coverArtMaxBytes {
		return zero, nil, false
	}
	data, err := readAllLimited(r, coverArtMaxBytes)
	if err != nil {
		return zero, nil, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > coverArtMaxPixels/cfg.Height {
		return zero, nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return zero, nil, false
	}
	dominant, ok := dominantCoverRGB(img)
	if !ok {
		return zero, nil, false
	}
	palette := paletteCoverRGB(img, want)
	// A black background usually wins the population vote on dark artwork, but
	// the visible subject (white line art, a logo, a moon) is not black. Keep
	// the subject's cluster instead of switching the whole indication off.
	if math.Max(dominant[0], math.Max(dominant[1], dominant[2])) < .12 {
		if visible, ok := visibleCoverRGB(img); ok {
			dominant = visible
		}
	} else if _, sat, _ := rgbHSV(dominant[0], dominant[1], dominant[2]); sat < coverNeutralCutoff {
		// A white or cream background can be the biggest cluster on a colourful
		// cover. Painting it leaves the whole ring pale, so a real colour wins
		// whenever the artwork has one. Neutral artwork keeps its neutral tone.
		if chromatic, ok := chromaticCoverRGB(img); ok {
			dominant = chromatic
		}
	}
	dominant = warmWhiteIfNeutral(dominant)
	return dominant, palette, true
}

// chromaticCoverRGB returns the most populated visibly coloured cluster.
// Clusters below half a percent are noise: one bright pixel must not repaint
// the ring. Colours are pushed away from grey the same way the palette is.
func chromaticCoverRGB(img image.Image) ([3]float64, bool) {
	entries, _, total := coverClusters(img)
	if total == 0 {
		return [3]float64{}, false
	}
	for _, e := range entries {
		v := math.Max(e.rgb[0], math.Max(e.rgb[1], e.rgb[2]))
		if v < .15 || e.count*200 < total {
			continue
		}
		if _, sat, _ := rgbHSV(e.rgb[0], e.rgb[1], e.rgb[2]); sat >= .25 {
			return boostCoverSaturation(e.rgb), true
		}
	}
	return [3]float64{}, false
}

// warmWhite turns "white" artwork into the colour the LEDs actually render as
// white. A neutral #FFFFFF drives all three channels, and on these rings the
// green and blue LEDs wash the result out; #FF6666 stays white but dimmer.
func warmWhiteIfNeutral(rgb [3]float64) [3]float64 {
	v := math.Max(rgb[0], math.Max(rgb[1], rgb[2]))
	// Grey artwork keeps its grey: only a bright neutral reads as "white" on
	// the ring, and only that one gets the LED-friendly warm white.
	if v < .75 {
		return rgb
	}
	if _, sat, _ := rgbHSV(rgb[0], rgb[1], rgb[2]); sat >= .10 {
		return rgb
	}
	return [3]float64{v, v * .40, v * .40}
}

// coverCell is one 8x8x8 colour bucket of the sampled artwork.
type coverCell struct {
	count int
	sum   [3]float64
}

// coverCluster is a populated bucket, ordered by population.
type coverCluster struct {
	count int
	rgb   [3]float64
}

// coverClusters samples the artwork into 8x8x8 buckets and returns them by
// population, the sampled pixels themselves and the sample count.
func coverClusters(img image.Image) ([]coverCluster, [][3]float64, int) {
	var bins [512]coverCell
	var points [][3]float64
	b := img.Bounds()
	sx, sy := b.Dx()/64, b.Dy()/64
	if sx < 1 {
		sx = 1
	}
	if sy < 1 {
		sy = 1
	}
	for y := b.Min.Y; y < b.Max.Y; y += sy {
		for x := b.Min.X; x < b.Max.X; x += sx {
			r, g, blue, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			// RGBA is premultiplied. Unpremultiply before quantising.
			p := [3]float64{float64(r) / float64(a), float64(g) / float64(a), float64(blue) / float64(a)}
			var q [3]int
			for k := range p {
				q[k] = int(p[k] * 8)
				if q[k] > 7 {
					q[k] = 7
				}
			}
			index := q[0]*64 + q[1]*8 + q[2]
			bins[index].count++
			for k := range p {
				bins[index].sum[k] += p[k]
			}
			points = append(points, p)
		}
	}
	entries := make([]coverCluster, 0, len(bins))
	total := 0
	for i := range bins {
		if bins[i].count == 0 {
			continue
		}
		var rgb [3]float64
		for k := range rgb {
			rgb[k] = bins[i].sum[k] / float64(bins[i].count)
		}
		entries = append(entries, coverCluster{count: bins[i].count, rgb: rgb})
		total += bins[i].count
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].count > entries[j].count })
	return entries, points, total
}

// visibleCoverRGB returns the most populated cluster that is not near-black.
// Stray pixels are ignored so one bright speck cannot repaint a black cover.
func visibleCoverRGB(img image.Image) ([3]float64, bool) {
	entries, _, total := coverClusters(img)
	if total == 0 {
		return [3]float64{}, false
	}
	for _, e := range entries {
		if math.Max(e.rgb[0], math.Max(e.rgb[1], e.rgb[2])) < .12 {
			continue
		}
		if e.count*200 < total {
			continue
		}
		return e.rgb, true
	}
	return [3]float64{}, false
}

// paletteCoverRGB keeps the most populated colour clusters of the artwork,
// dropping near-duplicates and near-black ones. The dominant cluster stays
// first, so a single-zone caller still gets the average colour.
func paletteCoverRGB(img image.Image, want int) [][3]float64 {
	if want <= 0 {
		return nil
	}
	entries, _, _ := coverClusters(img)
	// Album art is usually a photo on a white or cream background. Painting
	// that background would leave the whole indication pale, so chromatic
	// clusters win whenever the artwork has any; neutral ones are only used
	// for genuinely grayscale covers.
	var chromatic, neutral [][3]float64
	seen := func(list [][3]float64, rgb [3]float64) bool {
		for _, c := range list {
			d := 0.0
			for k := range c {
				d += (c[k] - rgb[k]) * (c[k] - rgb[k])
			}
			if d < .20*.20 {
				return true
			}
		}
		return false
	}
	for _, e := range entries {
		// Black is absence of light: the indication turns off instead of
		// inventing a dim tint for a dark cover.
		if math.Max(e.rgb[0], math.Max(e.rgb[1], e.rgb[2])) < .12 {
			continue
		}
		rgb := e.rgb
		colored := false
		if _, sat, _ := rgbHSV(rgb[0], rgb[1], rgb[2]); sat >= coverNeutralCutoff {
			rgb = boostCoverSaturation(rgb)
			colored = true
		}
		if colored {
			if seen(chromatic, e.rgb) {
				continue
			}
			chromatic = append(chromatic, rgb)
		} else if !seen(neutral, rgb) {
			neutral = append(neutral, warmWhiteIfNeutral(rgb))
		}
		if len(chromatic) >= want && len(neutral) >= want {
			break
		}
	}
	out := chromatic
	if len(out) == 0 {
		out = neutral
	}
	if len(out) > want {
		out = out[:want]
	}
	return out
}

// Select the most populated RGB cell, then average nearby pixels. All pixels
// vote equally, including white and black. Sparse bright logos cannot outweigh
// a neutral background just because the background has no hue.
func dominantCoverRGB(img image.Image) ([3]float64, bool) {
	entries, points, _ := coverClusters(img)
	if len(entries) == 0 {
		return [3]float64{}, false
	}
	center := entries[0].rgb
	var result [3]float64
	n := 0
	for _, p := range points {
		d := 0.0
		for k := range p {
			d += (p[k] - center[k]) * (p[k] - center[k])
		}
		if d > .16*.16 {
			continue
		}
		n++
		for k := range p {
			result[k] += p[k]
		}
	}
	for k := range result {
		result[k] /= float64(n)
	}
	if math.Max(result[0], math.Max(result[1], result[2])) < .12 {
		return [3]float64{}, true // black is off; no invented brightness floor
	}
	return result, true
}
