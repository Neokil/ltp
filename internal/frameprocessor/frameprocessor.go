package frameprocessor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"

	"github.com/Neokil/go-ext/pkg/slice"
)

type Tuple[K, V any] struct {
	V1 K
	V2 V
}

type ProcessorOptions struct {
	LineDirection      string // currently only horizontal is supported
	Lasercolor         color.Color
	MaxColorDeviation  uint16
	MinThroughWidth    int
	MinThroughHeight   uint16
	CalibrationResults CalibrationResults
	Debug              DebugOptions
}

type CalibrationResults struct {
	DistanceAt0  float64 // distance of laser lines at the plate (should be 0)
	DistanceAt10 float64 // distance of laser lines 10mm above the plate (the further apart, the better the height-calculation, but the smaller the resolution)
	WidthOfLaser float64 // thickness of the laser-line
	PixelPerMM   float64 // how many pixels represent one mm
}

type DebugOptions struct {
	Enable    bool
	Filenames map[string]string
}

func NewProcessorOptions() ProcessorOptions {
	return ProcessorOptions{
		LineDirection:      "horizontal",
		Lasercolor:         color.RGBA{R: 255, G: 0, B: 0, A: 255},
		MaxColorDeviation:  10000,
		MinThroughWidth:    15,
		MinThroughHeight:   1, // need to find a good default. Indicates how clear the line has to be to be recognized, should be more than the normal variance of colors
		CalibrationResults: CalibrationResults{},
	}
}

func (po ProcessorOptions) Validate() error {
	if po.LineDirection != "horizontal" {
		return fmt.Errorf("Line-Direction \"%s\" is invalid. Valid Values are: horizontal", po.LineDirection)
	}

	return nil
}

func ColorDistanceSimpleEuclidean(color1 color.Color, color2 color.Color) (uint16, error) {
	r1, g1, b1, _ := color1.RGBA()
	r2, g2, b2, _ := color2.RGBA()

	dist := math.Sqrt(
		math.Pow(float64(r1)-float64(r2), 2)+
			math.Pow(float64(g1)-float64(g2), 2)+
			math.Pow(float64(b1)-float64(b2), 2)) / 2 // max. value is 113509 so we scale it down to fit into uint16

	if dist < 0 {
		return 0, fmt.Errorf("dist is < 0 (%f) which should not be possible", dist)
	}
	if dist > math.MaxUint16 {
		if dist > 65601 {
			return 0, fmt.Errorf("dist is > max (%f) which should not be possible, c1[%d,%d,%d] c1[%d,%d,%d]", dist, r1, g1, b1, r2, g2, b2)
		}
		dist = math.MaxUint16
	}

	return uint16(dist), nil
}

func ColorDistanceRedman(color1 color.Color, color2 color.Color) (uint16, error) {
	r1Long, g1Long, b1Long, _ := color1.RGBA()
	r1 := float64(r1Long >> 8)
	g1 := float64(g1Long >> 8)
	b1 := float64(b1Long >> 8)
	r2Long, g2Long, b2Long, _ := color2.RGBA()
	r2 := float64(r2Long >> 8)
	g2 := float64(g2Long >> 8)
	b2 := float64(b2Long >> 8)

	r := 0.5 * (r1 + r2)
	dist := math.Sqrt((2+(r/256))*math.Pow(r1-r2, 2) + 4*math.Pow(g1-g2, 2) + (2 + ((255-r)/256)*math.Pow(b1-b2, 2)))

	// max is 674 and we need to scale that up to match uint16s 65535

	return uint16(dist * 65535 / 675), nil
}

func DetermineHeightPerLine(img image.Image, options ProcessorOptions) (map[int]float64, error) {
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate options")
	}

	result := map[int]float64{}

	debugImage := image.NewRGBA(image.Rect(0, 0, img.Bounds().Max.X, img.Bounds().Max.Y))

	minDiff := uint16(0)
	maxDiff := uint16(0)

	for y := range img.Bounds().Max.Y {
		pixels := []color.Color{}
		for x := range img.Bounds().Max.X {
			pixels = append(pixels, img.At(x, y))
		}
		diffToLaserColor, err := slice.ConvertWithErr(pixels, func(pixel color.Color) (uint16, error) {
			return ColorDistanceRedman(pixel, options.Lasercolor)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to calculate diff to laser color for line %d: %w", y, err)
		}

		for _, diff := range diffToLaserColor {
			if diff < minDiff {
				minDiff = diff
			}
			if diff > maxDiff {
				maxDiff = diff
			}
		}

		diffToLaserColor = slice.Convert(diffToLaserColor, func(f uint16) uint16 {
			if f > options.MaxColorDeviation {
				return math.MaxUint16
			}

			return f
		})
		for x := range len(diffToLaserColor) {
			debugImage.Set(x, y, color.RGBA{R: uint8(diffToLaserColor[x] >> 8), G: uint8(diffToLaserColor[x] >> 8), B: uint8(diffToLaserColor[x] >> 8), A: 255})
		}

		throughs, err := findThroughs(diffToLaserColor, options.MinThroughWidth, options.MinThroughHeight)
		if err != nil {
			return nil, fmt.Errorf("failed to find throughs: %w", err)
		}

		//for x := range throughs {
		//	debugImage.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		//}

		// what to do with the throughs?
		// check if there are 1 or two (more should be an error)
		// if 1 then we are at the gound level
		// if 2 calculate the height
		if len(throughs) == 1 {
			result[y] = 0.0

			continue
		}

		if len(throughs) == 2 {
			distBetweenPeaksInPixel := math.Abs(float64(throughs[0] - throughs[1]))
			distBetweenPeaksInMM := distBetweenPeaksInPixel / options.CalibrationResults.PixelPerMM
			result[y] = distBetweenPeaksInMM

			continue
		}

		result[y] = -1

		//return nil, fmt.Errorf("required 1 or 2 throughs but got %d for line %d (%v)", len(throughs), y, throughs)
	}

	fmt.Printf("MinDiff: %d, MaxDiff: %d\n", minDiff, maxDiff)

	if options.Debug.Enable {
		os.Remove(options.Debug.Filenames["debugimage"])
		f, err := os.OpenFile(options.Debug.Filenames["debugimage"], os.O_CREATE|os.O_WRONLY, 0x777)
		if err != nil {
			return nil, fmt.Errorf("failed to open debug file: %w", err)
		}
		err = jpeg.Encode(f, debugImage, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to encode debug image: %w", err)
		}
	}

	return result, nil
}

// analyzes the array of numbers and returns an array of throughs to find out
// where the color is closest to the color of the laser
func findThroughs(numbers []uint16, minThroughWidth int, minThroughHeight uint16) ([]int, error) {
	if minThroughWidth%2 != 1 {
		return nil, fmt.Errorf("the minimum through with needs to be an uneven number")
	}
	halfMinThroughWith := ((minThroughWidth - 1) / 2)

	throughs := []int{}
	// since we have to compare with numbers before we start at halfMinThroughWith
	for i := halfMinThroughWith; i < len(numbers)-halfMinThroughWith; i++ {
		if isThrough(numbers[i-halfMinThroughWith:i+halfMinThroughWith+1], halfMinThroughWith, minThroughHeight) {
			throughs = append(throughs, i)
		}
	}

	return throughs, nil
}

// check if we have a through.
// a through is defined as the center number being the highest value and the corners being the lowest value of their side
//
//	ideal through     through with noise
//
// ```
//
//	   xx         xx   x         x
//		 xx     xx      xx    x x
//		   xx xx          xx x x
//		     x              x
//
// ```
func isThrough(numbers []uint16, middleIndex int, minThroughHeight uint16) bool {
	centerValue := numbers[middleIndex]
	leftSideValue := numbers[0]
	rightSideValue := numbers[len(numbers)-1]

	// if the difference between middle and sides are too low it is not a through
	if int32(leftSideValue)-int32(centerValue) < int32(minThroughHeight) {
		return false
	}
	if int32(rightSideValue)-int32(centerValue) < int32(minThroughHeight) {
		return false
	}

	// check left side
	for i := middleIndex - 1; i > 0; i-- {
		// if we find a value that is lower than the center-point we do not have a through
		if numbers[i] < centerValue {
			return false
		}

		// if we find a value that is higher than the left side we do not have a through
		if numbers[i] > leftSideValue {
			return false
		}
	}

	// check right side
	for i := middleIndex + 1; i < len(numbers)-1; i++ {
		// if we find a value that is lower than the center-point we do not have a through
		if numbers[i] < centerValue {
			return false
		}

		// if we find a value that is higher than the right side we do not have a through
		if numbers[i] > rightSideValue {
			return false
		}
	}

	return true
}

func FrameToImage(frame []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewBuffer(frame))
	return img, err
}

func GaussianBlur(src image.Image, ksize float64) image.Image {
	// kernel of gaussian 15x15
	ks := int(ksize)
	k := make([]float64, ks*ks)
	for i := 0; i < ks; i++ {
		for j := 0; j < ks; j++ {
			k[i*ks+j] = math.Exp(-(math.Pow(float64(i)-ksize/2, 2)+math.Pow(float64(j)-ksize/2, 2))/(2*math.Pow(ksize/2, 2))) / 256
		}
	}

	// make an image that is ksize larger than the original
	dst := image.NewRGBA(src.Bounds())

	// apply
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			var r, g, b, a float64
			for ky := 0; ky < ks; ky++ {
				for kx := 0; kx < ks; kx++ {
					// get the source pixel
					c := src.At(x+kx-ks/2, y+ky-ks/2)
					r1, g1, b1, a1 := c.RGBA()
					// get the kernel value
					k := k[ky*ks+kx]
					// accumulate
					r += float64(r1) * k
					g += float64(g1) * k
					b += float64(b1) * k
					a += float64(a1) * k
				}
			}
			// set the destination pixel
			dst.Set(x, y, color.RGBA{uint8(r / 273), uint8(g / 273), uint8(b / 273), uint8(a / 273)})
		}
	}
	return dst
}
