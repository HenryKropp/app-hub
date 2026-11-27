//go:build ignore

// gerar writes the Agente's tray icons (ADR 049): a white "Z" in a circle,
// green when connected and grey otherwise. Run: go run gerar.go
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

func main() {
	write("conectado.ico", color.RGBA{0x16, 0xa3, 0x4a, 0xff})
	write("sem-conexao.ico", color.RGBA{0x9c, 0xa3, 0xaf, 0xff})
}

func write(name string, bg color.RGBA) {
	var pngs [][]byte
	sizes := []int{16, 32, 48}
	for _, n := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, draw(n, bg)); err != nil {
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

// draw renders the icon with 4x4 supersampling for smooth edges.
func draw(n int, bg color.RGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	const ss = 4
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var inCircle, inZ int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/ss) / float64(n)
					py := (float64(y) + (float64(sy)+0.5)/ss) / float64(n)
					if math.Hypot(px-0.5, py-0.5) <= 0.5 {
						inCircle++
						if isZ(px, py) {
							inZ++
						}
					}
				}
			}
			total := float64(ss * ss)
			a := float64(inCircle) / total
			if a == 0 {
				continue
			}
			z := float64(inZ) / float64(inCircle)
			mix := func(c uint8) uint8 { return uint8(float64(c)*(1-z) + 255*z) }
			img.SetNRGBA(x, y, color.NRGBA{mix(bg.R), mix(bg.G), mix(bg.B), uint8(a * 255)})
		}
	}
	return img
}

// isZ: top bar, bottom bar and the diagonal of a "Z" in unit coordinates.
func isZ(x, y float64) bool {
	const l, r, t, b, w = 0.28, 0.72, 0.27, 0.73, 0.13
	if x < l || x > r || y < t || y > b {
		return false
	}
	if y < t+w || y > b-w {
		return true
	}
	// Diagonal from top-right to bottom-left.
	u := (y - t) / (b - t)
	cx := r - u*(r-l)
	return math.Abs(x-cx) < w*0.75
}
