package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

func main() {
	// Read original appicon
	f, err := os.Open("build/appicon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}

	bounds := img.Bounds()

	// Generic tint function
	tint := func(name string, tintColor color.RGBA) {
		out := image.NewRGBA(bounds)
		// Draw original
		draw.Draw(out, bounds, img, image.Point{}, draw.Src)

		// Draw tint over it using Atop or standard blending
		// We'll just manually blend the pixels to preserve alpha
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r1, g1, b1, a1 := out.At(x, y).RGBA()
				if a1 == 0 {
					continue
				}
				// Convert back to 8-bit
				r, g, b, a := uint8(r1>>8), uint8(g1>>8), uint8(b1>>8), uint8(a1>>8)

				// Overlay tintColor with 50% opacity
				// dst = (src * alpha + dst * (1-alpha))
				alpha := 0.5
				nr := uint8(float64(tintColor.R)*alpha + float64(r)*(1-alpha))
				ng := uint8(float64(tintColor.G)*alpha + float64(g)*(1-alpha))
				nb := uint8(float64(tintColor.B)*alpha + float64(b)*(1-alpha))

				out.Set(x, y, color.RGBA{nr, ng, nb, a})
			}
		}

		outF, err := os.Create("internal/ui/tray/" + name)
		if err != nil {
			panic(err)
		}
		defer outF.Close()

		png.Encode(outF, out)
	}

	// Recording = Red
	tint("icon_recording.png", color.RGBA{255, 0, 0, 255})
	// Transcribing = Blue
	tint("icon_transcribing.png", color.RGBA{0, 150, 255, 255})
}
