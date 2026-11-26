//go:build panel

package video

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/HuskerMinion/techo5/echod/internal/component"
	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/media"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hook"
	"github.com/HuskerMinion/techo5/echod/internal/lib/panelctl"
	"github.com/HuskerMinion/techo5/echod/internal/lib/safe"
)

// The panel plays videos the way it shows the clock or opens an app: the app does it. This is the
// same Feature the Show's feature.go is — the same switches, sensors and actions, so Home Assistant,
// the DLNA renderer and Video Assistant cannot tell them apart — but instead of a session with a
// decoder inside it, a play is a line down the control line and the state is what the app tells
// back up it. The app is the one source of truth about what it is showing: this side only mirrors.
//
// On the control line (panelctl), echod says "video play <id> <url> <title>" ("" for no title),
// "video pause", "video resume" and "video stop"; the app answers "video state <id> <phase>",
// "video pos <id> <pos_s> <dur_s>", "video error <id> <text>" and "video ended <id>", and repeats
// the state of whatever it is showing whenever echod reconnects. An app from before these lines
// answers nothing, which is what the watchdog below is for.

// appAnswersIn is how long the app gets to say something about a video before the play is taken as
// failed: an app that died would leave the state loading forever. One that is alive but slow — a
// transcode finding its first frame — is left alone however long it takes; MediaPlayer's own error
// is the one that ends a stream that really cannot play.
const appAnswersIn = 15 * time.Second

// pausedFor is how long a video may stay paused before it is stopped: the Show's rule (feature.go),
// so a video left paused on either device frees itself the same way.
const pausedFor = 30 * time.Minute

// Feature is the panel's video player: a mirror of what the app is playing.
type Feature struct {
	on, dlna     *esphome.Switch
	state, title *esphome.TextSensor
	// lastErr is the Video error sensor: why the last video could not be played, empty once one plays.
	lastErr *esphome.TextSensor

	// Changed fires when there is something new to say. Listeners must not block.
	Changed hook.Hook[struct{}]

	mu     sync.Mutex
	seq    uint64
	cur    *State // the video up, as the app last said; nil with nothing playing
	acked  bool   // the app has said something about the video up: it is alive, however slow
	ended  uint64
	failed uint64
	err    string
	errAt  time.Time

	// publish is a nudge to the goroutine that tells Home Assistant.
	publish chan struct{}
}

// Frame is a picture of a video. The app draws its own picture over everything, so there are none
// here: Next never hands one out. The type is so the screen code compiles the same everywhere.
type Frame struct {
	Pix []byte
	N   int
}

var (
	once   sync.Once
	shared *Feature
)

// Get is the video player.
func Get() *Feature {
	once.Do(func() { shared = build() })
	return shared
}

func init() {
	if Here {
		component.Register(component.Device, Get(), component.Order(39))
	}
	panelctl.Handle("video", func(args []string) { Get().appSays(args) })
}

func build() *Feature {
	f := &Feature{publish: make(chan struct{}, 1)}
	f.on = &esphome.Switch{
		Base:      esphome.Base{ObjectID: "video", Name: "Video", Icon: "mdi:television-play", Category: esphome.CategoryConfig},
		OnCommand: f.SetOn,
	}
	f.dlna = &esphome.Switch{
		Base:      esphome.Base{ObjectID: "dlna_video", Name: "DLNA video", Icon: "mdi:cast", Category: esphome.CategoryConfig},
		OnCommand: f.SetDLNA,
	}
	f.state = &esphome.TextSensor{Base: esphome.Base{ObjectID: "video_state", Name: "Video state", Icon: "mdi:television-play"}}
	f.lastErr = &esphome.TextSensor{Base: esphome.Base{ObjectID: "video_error", Name: "Video error", Icon: "mdi:television-off"}}
	f.lastErr.Set("")
	f.title = &esphome.TextSensor{Base: esphome.Base{ObjectID: "video_title", Name: "Video title", Icon: "mdi:filmstrip"}}
	f.state.Set(string(Idle))
	f.title.Set("")
	f.Changed.Listen(func(struct{}) {
		select {
		case f.publish <- struct{}{}:
		default:
		}
	})
	safe.Go("video state", f.tell)
	return f
}

func (f *Feature) Name() string { return "video" }

func (f *Feature) Entities() []esphome.Entity {
	return []esphome.Entity{f.on, f.dlna, f.state, f.title, f.lastErr}
}

func (f *Feature) Restore(c config.Config) {
	f.on.Set(c.Video.On)
	f.dlna.Set(c.Video.DLNA)
	slog.Info("restored", "what", "video", "on", c.Video.On, "dlna", c.Video.DLNA)
}

// tell keeps Home Assistant's sensors up to date, and stops a video left paused too long: the same
// loop the Show runs, over the mirrored state instead of a session.
func (f *Feature) tell() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var last Phase
	var pausedID uint64
	var pausedAt time.Time
	for {
		select {
		case <-f.publish:
		case now := <-tick.C:
			st := f.State()
			switch {
			case st.Phase != Paused:
				pausedID = 0
			case st.ID != pausedID:
				pausedID, pausedAt = st.ID, now
			case now.Sub(pausedAt) > pausedFor:
				slog.Info("video: paused too long; stopped", "id", st.ID, "after", pausedFor)
				pausedID = 0
				f.StopID(st.ID)
			}
			if st.Phase == last || (!st.Active() && last == Idle) {
				continue
			}
			f.Changed.Emit(struct{}{})
		}
		st := f.State()
		last = st.Phase
		f.state.Set(string(st.Phase))
		if st.Phase == Playing {
			f.lastErr.Set("")
		}
		title := st.Title
		if title == "" {
			title = st.Host
		}
		if !st.Active() {
			title = ""
		}
		f.title.Set(title)
	}
}

// SetOn is the Video switch, from Home Assistant or the setup page. Off stops what is playing.
func (f *Feature) SetOn(on bool) {
	if err := config.Set().Video().On(on); err != nil {
		slog.Error("saving a setting failed", "setting", "video", "err", err)
		f.on.Set(!on)
		return
	}
	f.on.Set(on)
	slog.Info("setting changed", "setting", "video", "using", on)
	if !on {
		f.Stop()
	}
	f.Changed.Emit(struct{}{})
}

// SetDLNA is the DLNA video switch. Off stops a DLNA video that is playing.
func (f *Feature) SetDLNA(on bool) {
	if err := config.Set().Video().DLNA(on); err != nil {
		slog.Error("saving a setting failed", "setting", "dlna_video", "err", err)
		f.dlna.Set(!on)
		return
	}
	f.dlna.Set(on)
	slog.Info("setting changed", "setting", "dlna_video", "using", on)
	if !on {
		if st := f.State(); st.Active() && st.Origin == FromDLNA {
			f.Stop()
		}
	}
	f.Changed.Emit(struct{}{})
}

// Play asks the app to show a video, in place of whatever was up. The panel is a trusted home
// device and has no screen to ask a DLNA question on, so a DLNA video plays without asking: the
// DLNA video switch is the consent here.
func (f *Feature) Play(req Request) (uint64, error) {
	c := config.Get().Video
	if !c.On || (req.Origin == FromDLNA && !c.DLNA) {
		return 0, ErrOff
	}
	u, err := CheckURL(req.URL)
	if err != nil {
		return 0, err
	}
	if ownHost(u.Hostname()) {
		return 0, errors.New("the device does not fetch from itself")
	}
	title := strings.Join(strings.Fields(req.Title), " ") // the line carries words, not runs of space
	f.mu.Lock()
	f.seq++
	id := f.seq
	f.cur = &State{Phase: Loading, ID: id, Title: title, Origin: req.Origin, From: req.From, Host: u.Hostname()}
	f.acked = false
	f.mu.Unlock()
	// The video's sound is Android's media stream, so whatever the daemon was playing gives way to
	// it, as it does on the Show.
	media.Get().Stop()
	slog.Info("video: asking the app", "id", id, "url", Redacted(u), "origin", req.Origin, "from", req.From)
	if title == "" {
		title = `""`
	}
	panelctl.Tell("video play " + strconv.FormatUint(id, 10) + " " + u.String() + " " + title)
	f.Changed.Emit(struct{}{})
	time.AfterFunc(appAnswersIn, func() { f.stalled(id) })
	return id, nil
}

// stalled is a play the app said nothing about: it died. One that answered and is still loading is
// left to its own devices — a transcode can take a while, and MediaPlayer says so itself when a
// stream truly cannot play.
func (f *Feature) stalled(id uint64) {
	f.mu.Lock()
	if f.cur == nil || f.cur.ID != id || f.acked {
		f.mu.Unlock()
		return
	}
	f.cur, f.acked = nil, false
	f.failed, f.err, f.errAt = id, "video: the app did not answer", time.Now()
	f.mu.Unlock()
	f.lastErr.Set("video: the app did not answer")
	slog.Warn("video: the app said nothing about it; giving up", "id", id)
	f.Changed.Emit(struct{}{})
}

// Stop ends the video, whatever the app makes of that.
func (f *Feature) Stop() {
	f.mu.Lock()
	cur := f.cur
	f.cur = nil
	f.mu.Unlock()
	if cur != nil {
		slog.Info("video: stopped", "id", cur.ID)
	}
	panelctl.Tell("video stop")
	f.Changed.Emit(struct{}{})
}

// StopID ends video id if it is still the one up: a controller stopping its own video does not
// stop the next one somebody else started.
func (f *Feature) StopID(id uint64) {
	f.mu.Lock()
	cur := f.cur != nil && f.cur.ID == id
	f.mu.Unlock()
	if cur {
		f.Stop()
	}
}

// Pause and Resume ask the app to hold and carry on; the phase itself comes back from it.
func (f *Feature) Pause()  { panelctl.Tell("video pause") }
func (f *Feature) Resume() { panelctl.Tell("video resume") }

// Toggle pauses or resumes, whichever the video is not doing.
func (f *Feature) Toggle() {
	if st := f.State(); st.Phase == Paused {
		f.Resume()
	} else {
		f.Pause()
	}
}

// Covered is the display saying something is over the picture. On a panel the app's video is over
// everything already, and a call or a ring does not reach under it: nothing to pause under, nothing
// to carry on. (Debt: a real interruption would need a word to the app too.)
func (f *Feature) Covered(bool) {}

// Asking is the question on the screen: a panel never asks (Play).
func (f *Feature) Asking() (uint64, string, string, bool) { return 0, "", "", false }

// Answer is the screen's answer: a panel never asked.
func (f *Feature) Answer(uint64, bool) {}

// UseScreen is the display saying what the picture is for: the app takes the whole screen itself.
func (f *Feature) UseScreen(Screen, int, int) {}

// Next has no picture to hand out: the app draws its own.
func (f *Feature) Next() (*Frame, time.Duration, bool) { return nil, 0, false }

// Done takes no frame back.
func (f *Feature) Done(*Frame) {}

// State is the player now, as the app last said.
func (f *Feature) State() State {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := State{Phase: Idle, Ended: f.ended, Failed: f.failed, Err: f.err, ErrAt: f.errAt}
	if f.cur != nil {
		st = *f.cur
		st.Ended, st.Failed, st.Err, st.ErrAt = f.ended, f.failed, f.err, f.errAt
	}
	return st
}

// appSays is a line from the app that starts with "video".
func (f *Feature) appSays(args []string) {
	if len(args) < 2 {
		return
	}
	id, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return
	}
	f.mu.Lock()
	if f.cur != nil && f.cur.ID == id {
		f.acked = true // any word is a word: the app is alive
	}
	f.mu.Unlock()
	switch args[0] {
	case "state":
		if len(args) >= 3 {
			f.appState(id, Phase(args[2]))
		}
	case "pos":
		if len(args) >= 4 {
			f.appPos(id, args[2], args[3])
		}
	case "ended":
		f.appEnded(id)
	case "error":
		if len(args) >= 3 {
			f.appError(id, strings.Join(args[2:], " "))
		}
	}
}

// appState is the app saying what video id is doing. A word about a video this side does not know
// is adopted when nothing is up: echod restarted and the app is still showing something.
func (f *Feature) appState(id uint64, phase Phase) {
	switch phase {
	case Loading, Playing, Paused, Idle:
	default:
		return // a phase this build does not know: the app is newer, not wrong
	}
	f.mu.Lock()
	switch {
	case f.cur == nil && phase != Idle:
		f.cur = &State{ID: id, Phase: phase}
		slog.Info("video: the app is showing something already", "id", id, "phase", phase)
	case f.cur != nil && f.cur.ID == id:
		f.cur.Phase = phase
		if phase == Idle {
			f.cur = nil
		}
	default:
		f.mu.Unlock()
		return
	}
	f.mu.Unlock()
	f.Changed.Emit(struct{}{})
}

// appPos is the app telling where video id is. Home Assistant's Video sensor does not carry a
// position, so this only feeds State, for the DLNA renderer's GetPositionInfo.
func (f *Feature) appPos(id uint64, pos, dur string) {
	p, err1 := strconv.ParseFloat(pos, 64)
	d, err2 := strconv.ParseFloat(dur, 64)
	if err1 != nil || err2 != nil {
		return
	}
	f.mu.Lock()
	if f.cur != nil && f.cur.ID == id {
		f.cur.Pos, f.cur.Dur = time.Duration(p*float64(time.Second)), time.Duration(d*float64(time.Second))
	}
	f.mu.Unlock()
}

// appEnded is video id reaching its end by itself.
func (f *Feature) appEnded(id uint64) {
	f.mu.Lock()
	if f.cur == nil || f.cur.ID != id {
		f.mu.Unlock()
		return
	}
	f.cur, f.ended = nil, id
	f.mu.Unlock()
	slog.Info("video: over by itself", "id", id)
	f.Changed.Emit(struct{}{})
}

// appError is the app saying video id could not be played, or died playing it: the text goes to the
// error sensor, where Video Assistant reads it and decides what to re-normalize.
func (f *Feature) appError(id uint64, text string) {
	f.mu.Lock()
	f.failed, f.err, f.errAt = id, text, time.Now()
	current := f.cur != nil && f.cur.ID == id
	if current {
		f.cur = nil
	}
	f.mu.Unlock()
	f.lastErr.Set(clip(text, 250))
	slog.Warn("video: the app could not play it", "id", id, "err", text)
	if current {
		f.Changed.Emit(struct{}{})
	}
}

// The package's own calls, which the DLNA renderer and the setup page use on every device.

func Play(req Request) (uint64, error) { return Get().Play(req) }
func Stop()                            { Get().Stop() }
func StopID(id uint64)                 { Get().StopID(id) }
func Pause()                           { Get().Pause() }
func Resume()                          { Get().Resume() }
func Current() State                   { return Get().State() }
func SetOn(on bool)                    { Get().SetOn(on) }
func SetDLNA(on bool)                  { Get().SetDLNA(on) }
func Listen(fn func())                 { Get().Changed.Listen(func(struct{}) { fn() }) }

// DLNAOn is whether the DLNA renderer takes videos now. Android's own decoder is always installed
// here, so the two switches are all it takes.
func DLNAOn() bool {
	c := config.Get().Video
	return c.On && c.DLNA && Installed()
}

// Installed is whether this device can decode videos: on a panel the decoder is Android's own,
// as built in as the screen.
func Installed() bool { return true }
