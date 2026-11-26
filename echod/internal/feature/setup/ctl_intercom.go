//go:build intercom

package setup

import (
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/lib/panelctl"
)

// The intercom has no button to press to open the setup page, and its app is the only hand it
// has. The app asks over the control line ("setup"), the page opens as if a button had been
// pressed, and the app points a browser at it before the idle time runs out.
//
// A browser that comes in still has to be let in, the way a dot's button does it: the app cannot
// press, so this answers the asking itself for a few seconds after the request — the app that
// asked is the one opening the browser, and it is already this device's own app.

func init() {
	panelctl.Handle("setup", func(args []string) {
		Get().Open()
		panelctl.Tell("setup open")
		go letBrowserIn()
	})
}

// letBrowserIn waits for the browser the app is about to open to ask, and lets it in, the way a
// press would: for a few seconds only, so a stranger asking later still needs a real press.
func letBrowserIn() {
	for i := 0; i < 40; i++ {
		time.Sleep(250 * time.Millisecond)
		if asking := Get().Asking(); asking != "" {
			Get().AllowAsking(asking)
			return
		}
	}
}
