package frameprocessor

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"reflect"
	"testing"
)

func convertColorArrayToImage(pixels [][]color.Color, colorVariance int) image.Image {
	height := len(pixels)
	if height == 0 {
		return nil
	}
	width := len(pixels[0])

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for x := range width {
		for y := range height {
			pixelColor := pixels[y][x]

			if colorVariance > 0 {
				pixelColor = varyColor(pixelColor, colorVariance)
			}

			img.Set(x, y, pixelColor)
		}
	}

	return img
}
func varyColor(baseColor color.Color, variance int) color.RGBA {
	r, g, b, a := baseColor.RGBA()

	return color.RGBA{
		R: applyVariance(uint8(r>>8), variance),
		G: applyVariance(uint8(g>>8), variance),
		B: applyVariance(uint8(b>>8), variance),
		A: uint8(a >> 8),
	}
}

func applyVariance(value uint8, variance int) uint8 {
	offset := rand.Intn(2*variance) - int(variance)
	if offset+int(value) < 0 {
		return 0
	}
	if offset+int(value) > 255 {
		return 255
	}
	result := value + uint8(offset)

	//fmt.Printf("applying variance %d (actual offset %d) to %d = %d\n", variance, offset, value, result)
	return result
}

func getColorCodeDefinition(color color.Color) string {
	r, g, b, a := color.RGBA()

	return fmt.Sprintf("color.RGBA{R: %d, G: %d, B: %d, A: %d}", r>>8, g>>8, b>>8, a>>8)
}

func printImageDefinition(img image.Image) {
	fmt.Print("Image Definition:\n[][]color.Color{\n")
	for y := range img.Bounds().Max.Y {
		fmt.Print("  {\n")
		for x := range img.Bounds().Max.X {
			fmt.Print("    ", getColorCodeDefinition(img.At(x, y)), ",\n")
		}
		fmt.Print("  },\n")
	}
	fmt.Print("}")
}

var colorRed color.RGBA = color.RGBA{R: 255, G: 0, B: 0, A: 255}

func TestDetermineHeightPerLine(t *testing.T) {
	type args struct {
		img     image.Image
		options ProcessorOptions
	}
	tests := []struct {
		name    string
		args    args
		want    map[int]float64
		wantErr bool
		repeat  int
	}{
		{
			name: "basic test with clear colors, increasing distance and 1px laser",
			args: args{
				img: convertColorArrayToImage([][]color.Color{
					{color.Transparent, color.Transparent, color.Transparent, colorRed, color.Transparent, color.Transparent, color.Transparent},
					{color.Transparent, color.Transparent, colorRed, color.Transparent, colorRed, color.Transparent, color.Transparent},
					{color.Transparent, colorRed, color.Transparent, color.Transparent, color.Transparent, colorRed, color.Transparent},
				}, 0),
				options: ProcessorOptions{
					LineDirection:     "horizontal",
					Lasercolor:        colorRed,
					MaxColorDeviation: 10000,
					MinThroughWidth:   3,
					MinThroughHeight:  1,
					CalibrationResults: CalibrationResults{
						DistanceAt0:  0,
						DistanceAt10: 10,
						WidthOfLaser: 1,
						PixelPerMM:   1,
					},
				},
			},
			want: map[int]float64{
				0: 0,
				1: 2,
				2: 4,
			},
			wantErr: false,
		},
		{
			name: "basic test with varying colors, increasing distance and 1px laser",
			args: args{
				img: convertColorArrayToImage([][]color.Color{
					{
						color.RGBA{R: 9, G: 0, B: 3, A: 0},
						color.RGBA{R: 0, G: 2, B: 9, A: 0},
						color.RGBA{R: 0, G: 0, B: 9, A: 0},
						color.RGBA{R: 254, G: 0, B: 0, A: 255},
						color.RGBA{R: 3, G: 0, B: 7, A: 0},
						color.RGBA{R: 0, G: 8, B: 6, A: 0},
						color.RGBA{R: 1, G: 0, B: 0, A: 0},
					},
					{
						color.RGBA{R: 0, G: 0, B: 6, A: 0},
						color.RGBA{R: 8, G: 9, B: 1, A: 0},
						color.RGBA{R: 255, G: 0, B: 7, A: 255},
						color.RGBA{R: 0, G: 0, B: 3, A: 0},
						color.RGBA{R: 252, G: 9, B: 0, A: 255},
						color.RGBA{R: 1, G: 0, B: 8, A: 0},
						color.RGBA{R: 0, G: 0, B: 0, A: 0},
					},
					{
						color.RGBA{R: 0, G: 7, B: 2, A: 0},
						color.RGBA{R: 255, G: 7, B: 0, A: 255},
						color.RGBA{R: 0, G: 9, B: 5, A: 0},
						color.RGBA{R: 2, G: 2, B: 1, A: 0},
						color.RGBA{R: 0, G: 9, B: 0, A: 0},
						color.RGBA{R: 254, G: 0, B: 8, A: 255},
						color.RGBA{R: 9, G: 0, B: 4, A: 0},
					},
				}, 0),
				options: ProcessorOptions{
					LineDirection:     "horizontal",
					Lasercolor:        colorRed,
					MaxColorDeviation: 10000,
					MinThroughWidth:   3,
					MinThroughHeight:  1,
					CalibrationResults: CalibrationResults{
						DistanceAt0:  0,
						DistanceAt10: 10,
						WidthOfLaser: 1,
						PixelPerMM:   1,
					},
				},
			},
			want: map[int]float64{
				0: 0,
				1: 2,
				2: 4,
			},
			wantErr: false,
		},
		{
			name: "basic test with random varying colors, increasing distance and 1px laser",
			args: args{
				img: convertColorArrayToImage([][]color.Color{
					{color.Transparent, color.Transparent, color.Transparent, colorRed, color.Transparent, color.Transparent, color.Transparent},
					{color.Transparent, color.Transparent, colorRed, color.Transparent, colorRed, color.Transparent, color.Transparent},
					{color.Transparent, colorRed, color.Transparent, color.Transparent, color.Transparent, colorRed, color.Transparent},
				}, 10),
				options: ProcessorOptions{
					LineDirection:     "horizontal",
					Lasercolor:        colorRed,
					MaxColorDeviation: 10000,
					MinThroughWidth:   3,
					MinThroughHeight:  25,
					CalibrationResults: CalibrationResults{
						DistanceAt0:  0,
						DistanceAt10: 10,
						WidthOfLaser: 1,
						PixelPerMM:   1,
					},
				},
			},
			want: map[int]float64{
				0: 0,
				1: 2,
				2: 4,
			},
			wantErr: false,
			repeat:  10,
		},
	}
	for _, tt := range tests {
		if tt.repeat < 1 {
			tt.repeat = 1
		}
		for _ = range tt.repeat {
			t.Run(tt.name, func(t *testing.T) {
				got, err := DetermineHeightPerLine(tt.args.img, tt.args.options)
				if (err != nil) != tt.wantErr {
					printImageDefinition(tt.args.img)
					t.Errorf("DetermineHeightPerLine() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					printImageDefinition(tt.args.img)
					t.Errorf("DetermineHeightPerLine() = %v, want %v", got, tt.want)
				}
			})
		}
	}
}
