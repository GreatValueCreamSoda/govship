package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
)

type VideoSource struct {
	cmd       *exec.Cmd
	reader    *bufio.Reader
	frameSize int
}

func NewVideoSource(path string, width, height int) (*VideoSource, error) {
	frameSize := width*height + 2*(width*height/4) // YUV420p
	args := []string{"-loglevel", "panic", "-i", path, "-f", "rawvideo", "-pix_fmt", "yuv420p", "-"}
	cmd := exec.Command("ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &VideoSource{
		cmd:       cmd,
		reader:    bufio.NewReader(stdout),
		frameSize: frameSize,
	}, nil
}

// ReadFrame fills the provided buffer with one frame.
// Returns number of bytes read and error, handles EOF.
func (v *VideoSource) ReadFrame(buf []byte) (int, error) {
	if len(buf) != v.frameSize {
		return 0, fmt.Errorf("buffer size %d does not match frame size %d", len(buf), v.frameSize)
	}

	n, err := io.ReadFull(v.reader, buf)
	if err == io.ErrUnexpectedEOF {
		return n, io.EOF // treat incomplete frame as EOF
	}
	return n, err
}

func (v *VideoSource) Close() error {
	if err := v.cmd.Wait(); err != nil {
		return err
	}
	return nil
}
