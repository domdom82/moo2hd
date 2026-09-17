package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"golang.org/x/image/draw"
)

// imageToANSI renders img into an ANSI truecolor string using half-block characters (▄).
// Each terminal cell covers two image rows: background = top pixel, foreground = bottom pixel.
// The image is scaled up with nearest-neighbor to fill maxCols × maxRows terminal cells.
func imageToANSI(img image.Image, maxCols, maxRows int) string {
	if maxCols < 1 || maxRows < 1 {
		return ""
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return ""
	}

	// Scale factor: fit within terminal cell budget.
	// Each terminal row shows 2 pixel rows, so pixel height budget = maxRows*2.
	scaleW := maxCols / w
	scaleH := (maxRows * 2) / h
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	if scale < 1 {
		scale = 1
	}

	scaledW := w * scale
	scaledH := h * scale

	scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
	draw.NearestNeighbor.Scale(scaled, scaled.Bounds(), img, bounds, draw.Src, nil)

	var sb strings.Builder
	for y := 0; y < scaledH; y += 2 {
		for x := range scaledW {
			top := scaled.RGBAAt(x, y)
			var bot color.RGBA
			if y+1 < scaledH {
				bot = scaled.RGBAAt(x, y+1)
			}

			if top.A == 0 && bot.A == 0 {
				sb.WriteString("\033[0m ")
				continue
			}
			if top.A == 0 {
				fmt.Fprintf(&sb, "\033[0m\033[38;2;%d;%d;%dm▄", bot.R, bot.G, bot.B)
				continue
			}
			if bot.A == 0 {
				fmt.Fprintf(&sb, "\033[48;2;%d;%d;%dm ", top.R, top.G, top.B)
				continue
			}
			fmt.Fprintf(&sb, "\033[48;2;%d;%d;%dm\033[38;2;%d;%d;%dm▄",
				top.R, top.G, top.B, bot.R, bot.G, bot.B)
		}
		sb.WriteString("\033[0m\n")
	}
	return sb.String()
}
