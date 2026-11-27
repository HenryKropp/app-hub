//go:build ignore

// gerar writes the Agente's tray icons (ADR 049): the Ziv icon (petrol
// square, sand "i") with a status dot in the corner. Connected: coral "i" dot
// (the lighthouse on) and a green status dot. Disconnected: both grey.
// Run: go run gerar.go
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

var (
	petrol = color.RGBA{0x0e, 0x3b, 0x43, 0xff}
	sand   = color.RGBA{0xf4, 0xef, 0xe6, 0xff}
	coral  = color.RGBA{0xff, 0x6b, 0x4a, 0xff}
	white  = color.RGBA{0xff, 0xff, 0xff, 0xff}
	green  = color.RGBA{0x16, 0xa3, 0x4a, 0xff}
	grey   = color.RGBA{0x9c, 0xa3, 0xaf, 0xff}
	dark   = color.RGBA{0x6b, 0x72, 0x80, 0xff}
)

func main() {
	write("conectado.ico", coral, green)
	write("sem-conexao.ico", dark, grey)
}

func write(name string, dot, status color.RGBA) {
	var pngs [][]byte
	sizes := []int{16, 32, 48}
	for _, n := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, draw(n, dot, status)); err != nil {
			panic(err)
		}
		pngs = append(pngs, buf.Bytes())
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, n := range sizes {
		binary.Write(&out, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, Bits           uint16
			Size, Offset           uint32
		}{uint8(n), uint8(n), 0, 0, 1, 32, uint32(len(pngs[i])), uint32(offset)})
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		out.Write(p)
	}
	if err := os.WriteFile(name, out.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

// shape is a filled area in 48x48 icon units (the Logo's viewBox in bits.tsx).
type shape struct {
	in func(x, y float64) bool
	c  color.RGBA
}

// draw paints the shapes in order, with 4x4 supersampling for smooth edges.
func draw(n int, dot, status color.RGBA) *image.NRGBA {
	shapes := []shape{
		{roundedSquare, petrol},
		{capsule(24, 22, 37, 3), sand}, // the i's stem
		{circle(24, 13.5, 4.6), dot},   // the i's dot
		{circle(38, 38, 10.5), white},  // ring around the status dot
		{circle(38, 38, 9), status},    // status dot
	}
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	const ss = 4
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/ss) * 48 / float64(n)
					py := (float64(y) + (float64(sy)+0.5)/ss) * 48 / float64(n)
					var c *color.RGBA
					for i := range shapes {
						if shapes[i].in(px, py) {
							c = &shapes[i].c
						}
					}
					if c != nil {
						r, g, b, a = r+float64(c.R), g+float64(c.G), b+float64(c.B), a+1
					}
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(a / (ss * ss) * 255)})
		}
	}
	return img
}

// roundedSquare is the 48x48 square with 12-unit corners.
func roundedSquare(x, y float64) bool {
	const r = 12
	cx := math.Max(r, math.Min(48-r, x))
	cy := math.Max(r, math.Min(48-r, y))
	return math.Hypot(x-cx, y-cy) <= r
}

func circle(cx, cy, r float64) func(x, y float64) bool {
	return func(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= r }
}

// capsule is a vertical stroke with round caps, from y1 to y2 at x, half-width w.
func capsule(x0, y1, y2, w float64) func(x, y float64) bool {
	return func(x, y float64) bool {
		cy := math.Max(y1, math.Min(y2, y))
		return math.Hypot(x-x0, y-cy) <= w
	}
}
