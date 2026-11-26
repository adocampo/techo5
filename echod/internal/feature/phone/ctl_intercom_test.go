//go:build intercom

package phone

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/component"
	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/announce"
	"github.com/HuskerMinion/techo5/echod/internal/lib/panelctl"
)

// The control socket's name belongs to the machine, and every test binary that imports this
// package dials it from the bridge's init. Take a name of this process alone so the tests below
// meet their own echod and not a neighbour test binary's.
func TestMain(m *testing.M) {
	panelctl.Socket = fmt.Sprintf("@techo5-ctl-test-%d", os.Getpid())
	os.Exit(m.Run())
}

// ctlApp is a fake app for a test: it serves the control socket the way the app does and hands over
// the lines echod sends. One at a time: the socket name belongs to the machine, not to the test.
type ctlApp struct {
	t *testing.T
	c net.Conn
	r *bufio.Reader
}

func startCtlApp(t *testing.T) *ctlApp {
	t.Helper()
	ln, err := net.Listen("unix", panelctl.Socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	type incoming struct {
		c   net.Conn
		err error
	}
	got := make(chan incoming, 1)
	go func() {
		c, err := ln.Accept()
		got <- incoming{c, err}
	}()
	// panelctl dials again every second, so this only waits out the gap between tries.
	select {
	case a := <-got:
		if a.err != nil {
			t.Fatal(a.err)
		}
		// Let go of the connection too: panelctl keeps one and only dials again once the old one
		// breaks, so a test that leaves it open leaves the next one waiting for a caller that is
		// already connected somewhere else.
		t.Cleanup(func() { _ = a.c.Close() })
		return &ctlApp{t: t, c: a.c, r: bufio.NewReader(a.c)}
	case <-time.After(5 * time.Second):
		t.Fatal("echod never dialed the control socket")
		return nil
	}
}

// line is the next line echod sent that starts with want, skipping the rest: panelctl replays the
// latest of each kind on a new connection, and other lines cross the wire between the ones a test
// waits for.
func (a *ctlApp) line(want string) string {
	a.t.Helper()
	_ = a.c.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		s, err := a.r.ReadString('\n')
		if err != nil {
			a.t.Fatalf("reading the control line %q: %v", want, err)
		}
		if s = strings.TrimSpace(s); strings.HasPrefix(s, want) {
			return s
		}
	}
}

func (a *ctlApp) send(line string) {
	_, _ = a.c.Write([]byte(line + "\n"))
}

// A call from another device rings, the app is told who it is, and answering it from the app
// connects it: the whole round trip the intercom app lives on, over the real control socket.
func TestCtlCallAnsweredByTheApp(t *testing.T) {
	p, peer := house(t)
	app := startCtlApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l, err := dialDevice(ctx, peer, intercomKey())
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	if got := <-l.control; got != msgRinging {
		t.Fatalf("first answer %q, not ringing", got)
	}
	caller := shownName([]byte(config.Get().Device.Name))
	if got := app.line("call in "); got != "call in "+caller {
		t.Fatalf("the app was told %q", got)
	}
	app.send("call answer")
	waitFor(t, "the call to be answered", func() bool { return p.State().Phase == Talking })
	select {
	case got := <-l.control:
		if got != msgAnswer {
			t.Fatalf("the caller was sent %q, not answer", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the caller never heard the call was answered")
	}
	if got := app.line("call answered"); got != "call answered" {
		t.Fatalf("the app was told %q", got)
	}
	p.Hangup()
	if got := app.line("call ended"); got != "call ended" {
		t.Fatalf("the app was told %q", got)
	}
}

// The app asks for the house and gets it as the plan's lines: one peer per device and an end.
func TestCtlPeersList(t *testing.T) {
	house(t)
	app := startCtlApp(t)
	old := ctlPeers
	ctlPeers = func() []announce.Peer {
		return []announce.Peer{
			{Name: "Kitchen", Address: "192.168.1.4", Port: 8181},
			{Name: "Big Room", Address: "192.168.1.9", Port: 8181},
		}
	}
	t.Cleanup(func() { ctlPeers = old })

	app.send("peers")
	if got := app.line("peer "); got != "peer Kitchen 192.168.1.4:8181" {
		t.Fatalf("first peer line %q", got)
	}
	if got := app.line("peer "); got != "peer Big Room 192.168.1.9:8181" {
		t.Fatalf("second peer line %q", got)
	}
	if got := app.line("peers end"); got != "peers end" {
		t.Fatalf("the list did not end: %q", got)
	}
}

// An announcement reaches the app on one line, however it was spoken.
func TestCtlAnnounceLine(t *testing.T) {
	house(t)
	app := startCtlApp(t)
	announce.Get().Arrived.Emit(announce.Message{From: "Angel", Text: "Dinner is\nready, come down"})
	if got := app.line("announce "); got != "announce Dinner is ready, come down" {
		t.Fatalf("the app was told %q", got)
	}
}

// The toggles travel both ways: the app moves them, and a move from Home Assistant comes back as
// the Changed emit the switches already fire.
func TestCtlToggles(t *testing.T) {
	house(t)
	app := startCtlApp(t)
	app.send("dnd 1")
	if got := app.line("dnd 1"); got != "dnd 1" {
		t.Fatalf("the app was told %q", got)
	}
	if !config.Get().Home.DoNotDisturb {
		t.Fatal("do not disturb did not take")
	}
	Get().SetDropIn(true) // as Home Assistant's switch would
	if got := app.line("dropin 1"); got != "dropin 1" {
		t.Fatalf("the app was told %q", got)
	}
}

// A call the house refused while busy is told to the app as its own line.
func TestCtlCallBusy(t *testing.T) {
	house(t)
	app := startCtlApp(t)
	component.Fire.Emit(component.Event{Name: Event, Data: map[string]string{
		"event": "not_answered", "kind": "intercom", "reason": "busy", "peer": "Kitchen",
	}})
	if got := app.line("call busy"); got != "call busy" {
		t.Fatalf("the app was told %q", got)
	}
}
