package videoreader

import (
	"fmt"

	vidio "github.com/AlexEidt/Vidio"
)

var EOF error = fmt.Errorf("EOF")

type VideoReader interface {
	Read(filename string) (VideoHandle, error)
}

type VideoHandle interface {
	GetNextFrame() ([]byte, error)
}

type videoReader struct {
	v *vidio.Video
}

func New() VideoReader {
	return &videoReader{}
}

func (vr *videoReader) Read(filename string) (VideoHandle, error) {
	var err error
	vr.v, err = vidio.NewVideo(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read video from file: %w", err)
	}

	return vr, nil
}

func (vr *videoReader) GetNextFrame() ([]byte, error) {
	if vr.v.Read() {
		return vr.v.FrameBuffer(), nil
	}

	return nil, EOF
}
