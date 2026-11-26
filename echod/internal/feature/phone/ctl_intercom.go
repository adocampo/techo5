//go:build intercom

package phone

import (
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/HuskerMinion/techo5/echod/internal/component"
	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/announce"
	"github.com/HuskerMinion/techo5/echod/internal/lib/panelctl"
)

// The app's side of the intercom: every call goes to the app as one line ("call in Kitchen"), the
// house's other devices on request ("peer Kitchen 192.168.1.4:8181" … "peers end"), announcements
// as they arrive ("announce Dinner is ready"), and the app's taps come back as commands
// ("call answer", "call Kitchen", "dnd 1"). The intercom app draws its screens from these lines;
// docs/plan.md in techo5-intercom has the app's side.
//
// panelctl keeps the last line of each kind and sends it again on every connect, so an app that
// restarted mid-call is told where the call stands. The peers and the announcements are news, not
// state, and go through Tell.

// ctlPeers is announce's list, a variable so a test can furnish a house.
var ctlPeers = announce.Peers

func init() {
	var mu sync.Mutex
	var last Phase
	var lastIntercom bool

	Get().Changed.Listen(func(st State) {
		// set() clears Intercom the moment the phase is Idle, so whether the call that just ended
		// was an intercom one is remembered from the line before.
		line := ""
		mu.Lock()
		switch {
		case st.Intercom && st.Phase == Ringing && st.Incoming:
			line = "call in " + st.Peer
		case st.Intercom && (st.Phase == Ringing || st.Phase == Dialing):
			line = "call out " + st.Peer
		case st.Intercom && st.Phase == Talking:
			line = "call answered"
		case st.Phase == Idle && last != Idle && lastIntercom:
			line = "call ended"
		}
		last, lastIntercom = st.Phase, st.Intercom
		mu.Unlock()
		if line != "" {
			panelctl.Send(line)
		}
		// The toggles ride every emit too: SetDoNotDisturb and SetDropIn fire Changed themselves, so
		// the app is told however the switch was moved — its own, Home Assistant's, or a restore.
		home := config.Get().Home
		panelctl.Send("dnd " + onOff(home.DoNotDisturb))
		panelctl.Send("dropin " + onOff(home.DropIn))
	})

	// A call the house refused while busy is worth a line of its own: the app can say why the call
	// it just placed is already over.
	component.Fire.Listen(func(ev component.Event) {
		if ev.Name == Event && ev.Data["event"] == "not_answered" &&
			ev.Data["kind"] == "intercom" && ev.Data["reason"] == "busy" {
			panelctl.Tell("call busy")
		}
	})

	announce.Get().Arrived.Listen(func(m announce.Message) {
		if m.Kind != "" {
			return // a reminder between devices is not a communiqué for the app to show
		}
		panelctl.Tell("announce " + oneLine(m.Text))
	})

	panelctl.Handle("call", func(args []string) {
		p := Get()
		switch strings.Join(args, " ") {
		case "answer":
			p.Answer()
		case "hangup":
			p.Hangup()
		default:
			if len(args) > 0 {
				if err := p.CallDevice(strings.Join(args, " ")); err != nil {
					slog.Info("intercom: the app asked for a call that did not start", "err", err)
				}
			}
		}
	})

	panelctl.Handle("peers", func([]string) {
		for _, pe := range ctlPeers() {
			panelctl.Tell("peer " + pe.Name + " " + net.JoinHostPort(pe.Address, strconv.Itoa(pe.Port)))
		}
		panelctl.Tell("peers end")
	})

	panelctl.Handle("dnd", func(args []string) {
		Get().SetDoNotDisturb(on(args))
	})
	panelctl.Handle("dropin", func(args []string) {
		Get().SetDropIn(on(args))
	})
}

func on(args []string) bool { return len(args) > 0 && args[0] == "1" }

func onOff(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// oneLine keeps a spoken announcement to the single line the app's notification shows.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
