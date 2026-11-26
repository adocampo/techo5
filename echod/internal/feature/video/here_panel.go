//go:build panel

package video

// Here is whether this device plays videos: a panel does. The decoder is Android's own — the app
// plays what echod asks for over the control line (panel.go), the way the clock and the apps run
// there — so no techo5-ffmpeg is needed. The switches, the play_video actions and the sensors are
// the Show's; the playing is the app's.
const Here = true
