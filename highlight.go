package main

import (
	"fmt"
	"image"
	"math"
)

// highlight leaves the part as it is and pushes the rest of the picture back, so that the eye goes to the part. Unlike the other effects it changes what
// is outside the part. The level says how far: light only shades the rest a little; medium makes it darker and paler, with a soft shadow round the part,
// and leaves it readable; full (the default) also blurs it. The part always gets a thin light border with a faint dark edge outside it (so that it shows
// on a light picture too). amount scales the level (0 to 100, default 60); color is the border (#rrggbb or #rrggbbaa, white by default; a transparent
// color leaves the border out).
func highlight(base *image.NRGBA, sh *shape, sp effectSpec) (string, error) {
	b := base.Bounds()
	w, h := b.Dx(), b.Dy()
	in := make([]float32, w*h)
	n := 0
	for y := max(sh.r.Min.Y, b.Min.Y); y < min(sh.r.Max.Y, b.Max.Y); y++ {
		for x := max(sh.r.Min.X, b.Min.X); x < min(sh.r.Max.X, b.Max.X); x++ {
			if sh.in(x, y) {
				in[(y-b.Min.Y)*w+x-b.Min.X] = 1
				n++
			}
		}
	}
	if n == 0 {
		return "", fmt.Errorf("the part is outside the image")
	}
	if n == w*h {
		return "", fmt.Errorf("select the part to highlight first: highlight pushes back the rest of the image")
	}
	border, err := parseColorA(sp.Color, "#FFFFFFE6")
	if err != nil {
		return "", err
	}
	s := math.Max(0, math.Min(1, sp.amount(60)/100))
	side := float64(min(w, h))
	var radius, dark, gray, shade float64 // the blur in pixels, and how much darker, paler and shadowed the rest is (0 to 1)
	switch sp.Level {
	case "light":
		dark = 0.2 * s
	case "medium":
		dark, gray, shade = 0.4*s, 0.4*s, 0.5*s
	case "", "full":
		radius, dark, gray, shade = math.Max(2, side/150)*(0.5+s), 0.75*s, s, s
	default:
		return "", fmt.Errorf("the level of highlight is light, medium or full, not %q", sp.Level)
	}

	// the rest: blurred, paler (less color) and darker
	rest := cloneNRGBA(base)
	if radius > 0 {
		blur(rest, radius)
	}
	dim := 1 - dark
	each(rest, func(r, g, bl float64) (float64, float64, float64) {
		l := lum(r, g, bl)
		return (r + (l-r)*gray) * dim, (g + (l-g)*gray) * dim, (bl + (l-bl)*gray) * dim
	})

	// the shadow: the part, moved a little down and right and made soft; it darkens the rest round the part
	k := int(math.Max(3, side/70))
	off := max(1, k/3)
	shadow := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sx, sy := x-off, y-off; sx >= 0 && sy >= 0 {
				shadow[y*w+x] = in[sy*w+sx]
			}
		}
	}
	shadow = boxBlur(shadow, w, h, k)

	// the border: what is outside the part but within bw pixels of it (the coverage of a small blur is above 0 there)
	bw := int(math.Max(1, math.Round(side/350)))
	ring := boxBlur(append([]float32(nil), in...), w, h, bw)
	edge := boxBlur(append([]float32(nil), in...), w, h, 2*bw+1) // a little wider: the dark edge outside the border

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := y*w + x
			if in[p] >= 1 {
				continue
			}
			i := y*base.Stride + 4*x
			for c := 0; c < 3; c++ {
				v := float32(rest.Pix[i+c]) * (1 - 0.6*float32(shade)*shadow[p])
				base.Pix[i+c] = clamp8(float64(v))
			}
			base.Pix[i+3] = rest.Pix[i+3]
			if a := float32(border.A) / 255 * 0.45 * min(1, edge[p]*4) * (1 - min(1, ring[p]*4)); a > 0.003 {
				base.Pix[i], base.Pix[i+1], base.Pix[i+2] = lerp8(base.Pix[i], 0, a), lerp8(base.Pix[i+1], 0, a), lerp8(base.Pix[i+2], 0, a)
			}
			if a := float32(border.A) / 255 * min(1, ring[p]*4); a > 0.003 {
				base.Pix[i], base.Pix[i+1], base.Pix[i+2] = lerp8(base.Pix[i], border.R, a), lerp8(base.Pix[i+1], border.G, a), lerp8(base.Pix[i+2], border.B, a)
				base.Pix[i+3] = lerp8(base.Pix[i+3], 255, a)
			}
		}
	}
	if sp.Level == "" {
		sp.Level = "full"
	}
	return fmt.Sprintf("highlighted %d × %d pixels at %d,%d (%s: the rest is pushed back)", sh.r.Dx(), sh.r.Dy(), sh.r.Min.X, sh.r.Min.Y, sp.Level), nil
}
