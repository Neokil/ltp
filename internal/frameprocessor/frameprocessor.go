package frameprocessor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"

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
}

type CalibrationResults struct {
	DistanceAt0  float64 // distance of laser lines at the plate (should be 0)
	DistanceAt10 float64 // distance of laser lines 10mm above the plate (the further apart, the better the height-calculation, but the smaller the resolution)
	WidthOfLaser float64 // thickness of the laser-line
	PixelPerMM   float64 // how many pixels represent one mm
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

func DetermineHeightPerLine(img image.Image, options ProcessorOptions) (map[int]float64, error) {
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate options")
	}

	result := map[int]float64{}

	for y := range img.Bounds().Max.Y {
		pixels := []color.Color{}
		for x := range img.Bounds().Max.X {
			pixels = append(pixels, img.At(x, y))
		}
		diffToLaserColor1 := slice.Convert(pixels, func(pixel color.Color) uint16 {
			r, g, b, _ := pixel.RGBA()
			laserR, laserG, laserB, _ := options.Lasercolor.RGBA()

			dist := math.Sqrt(
				math.Pow(math.Abs(float64(r)-float64(laserR)), 2) +
					math.Pow(math.Abs(float64(g)-float64(laserG)), 2) +
					math.Pow(math.Abs(float64(b)-float64(laserB)), 2))

			if dist < 0 {
				dist = 0
			}
			if dist > math.MaxUint16 {
				dist = math.MaxUint16
			}

			return uint16(dist)
		})
		diffToLaserColor2 := slice.Convert(diffToLaserColor1, func(f uint16) uint16 {
			if f > options.MaxColorDeviation {
				return math.MaxUint16
			}

			return f
		})

		throughs, err := findThroughs(diffToLaserColor2, options.MinThroughWidth, options.MinThroughHeight)
		if err != nil {
			return nil, fmt.Errorf("failed to find throughs: %w", err)
		}

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

		return nil, fmt.Errorf("required 1 or 2 throughs but got %d for line %d (%v)", len(throughs), y, throughs)
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
	// since we have to compare with number before we start at 1
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

func GaussianBlur(src *image.RGBA, ksize float64) *image.RGBA {
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
