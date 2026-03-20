package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

func main() {
	sizes := []int{192, 512}
	for _, size := range sizes {
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		
		// Fill background with #6d28d9
		purple := color.RGBA{0x6d, 0x28, 0xd9, 0xff}
		draw.Draw(img, img.Bounds(), &image.Uniform{purple}, image.Point{}, draw.Src)
		
		// Draw a simple white "H"
		// This is very basic, but fulfills the requirement "simple white H"
		white := color.RGBA{255, 255, 255, 255}
		
		// Thickness and dimensions for H
		thickness := size / 8
		margin := size / 4
		
		// Left bar
		draw.Draw(img, image.Rect(margin, margin, margin+thickness, size-margin), &image.Uniform{white}, image.Point{}, draw.Over)
		// Right bar
		draw.Draw(img, image.Rect(size-margin-thickness, margin, size-margin, size-margin), &image.Uniform{white}, image.Point{}, draw.Over)
		// Middle bar
		draw.Draw(img, image.Rect(margin, size/2-thickness/2, size-margin, size/2+thickness/2), &image.Uniform{white}, image.Point{}, draw.Over)

		outPath := filepath.Join("static", "icon-"+string(itoa(size))+".png")
		if size == 192 {
			outPath = "static/icon-192.png"
		} else {
			outPath = "static/icon-512.png"
		}

		f, err := os.Create(outPath)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		log.Printf("Generated %s", outPath)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	res := ""
	for n > 0 {
		res = string(rune('0'+(n%10))) + res
		n /= 10
	}
	return res
}
