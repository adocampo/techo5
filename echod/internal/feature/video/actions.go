//go:build !dot

package video

import (
	"fmt"
	"strings"

	esphome "github.com/ygelfand/go-esphome-device"
)

// Actions are the four the ESPHome link offers: the same on every device that plays videos, so they
// live here rather than in the Show's feature.go or the panel's panel.go. The panel plays through
// its app; from Home Assistant it cannot tell.

func (f *Feature) Actions() []*esphome.Action {
	return []*esphome.Action{
		{
			// Plays a video full screen: an http or https address (MP4, MKV, MPEG-TS or HLS, H.264 at
			// 720p or less plays best), with an optional title for the screen and the sensor.
			Name: "play_video",
			Args: []esphome.Arg{{Name: "url", Type: esphome.ArgString}, {Name: "title", Type: esphome.ArgString}},
			Run: func(c esphome.Call) (any, error) {
				_, err := f.Play(Request{URL: c.String("url"), Title: c.String("title"), Origin: FromHomeAssistant})
				if err != nil && !strings.HasPrefix(err.Error(), "video: ") {
					err = fmt.Errorf("video: %w", err)
				}
				if err != nil {
					f.lastErr.Set(clip(err.Error(), 250))
				}
				return nil, err
			},
		},
		{Name: "stop_video", Run: func(esphome.Call) (any, error) { f.Stop(); return nil, nil }},
		{Name: "pause_video", Run: func(esphome.Call) (any, error) { f.Pause(); return nil, nil }},
		{Name: "resume_video", Run: func(esphome.Call) (any, error) { f.Resume(); return nil, nil }},
	}
}

// clip cuts a message to what a sensor can carry.
func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
