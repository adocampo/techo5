//go:build panel

package video

import (
	"context"
	"image"
	"io"
	"time"
)

// The panel has no ffmpeg: a camera stream, which the Show decodes with it, is not for it. The app
// bridges its own camera the other way (the front camera out to Home Assistant); nothing decodes a
// remote camera here. Same answers as the Dot's (camera_dot.go), with the error that says why: the
// image has no decoder, though it does play videos through the app.
type Camera struct{}

func OpenCamera(context.Context, string, int, int) (*Camera, error) { return nil, ErrNotInstalled }
func (*Camera) Next(time.Duration) (*image.RGBA, error)             { return nil, ErrNotInstalled }
func (*Camera) Close()                                              {}

func OpenCameraSound(context.Context, string) (io.ReadCloser, error) { return nil, ErrNotInstalled }
func CameraLiveChanged()                                             {}

// CanRTSP is whether this device can take an RTSP stream: that needs the decoder, which a panel
// does not carry.
func CanRTSP() bool { return false }

func OpenCameraSoundRTSP(context.Context, string, string, string) (io.ReadCloser, error) {
	return nil, ErrNotInstalled
}
func OpenCameraRTSP(context.Context, string, string, string, int, int) (*Camera, error) {
	return nil, ErrNotInstalled
}
func OpenCameraRTSPFilling(context.Context, string, string, string, int, int) (*Camera, error) {
	return nil, ErrNotInstalled
}
func OpenCameraRTSPFitting(context.Context, string, string, string, int, int) (*Camera, error) {
	return nil, ErrNotInstalled
}
func OpenCameraRTSPCovering(context.Context, string, string, string, int, int) (*Camera, error) {
	return nil, ErrNotInstalled
}
func OpenCameraCovering(context.Context, string, int, int) (*Camera, error) {
	return nil, ErrNotInstalled
}
