//go:build panel

package layout

import "testing"

// A panel is no Echo board: main's Show code asks these to pick athena's or the Show 8's paths, and
// on a tablet every one of them has to say no, or the daemon would go looking for hardware an app
// cannot reach.
func TestPanelIsNoShowBoard(t *testing.T) {
	for name, got := range map[string]bool{
		"Checkers":       Checkers(),
		"Crown":          Crown(),
		"Show8":          Show8(),
		"Athena":         Athena(),
		"AthenaMainline": AthenaMainline(),
	} {
		if got {
			t.Errorf("%s() = true on a panel", name)
		}
	}
	if v, ok := idmeFallback("serial"); ok || v != "" {
		t.Errorf("idmeFallback = %q, %v on a panel; want nothing", v, ok)
	}
}

// The state directory is the files directory the app started echod in, whichever user of the tablet
// it belongs to; anything that is not an app files directory falls back to the first user's. The
// app's package is the build's own, so this reads the same for a panel and for an intercom.
func TestStateDirIsTheAppsFilesDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	fallback := "/data/user/0/" + PackageName + "/files"
	own := map[string]string{
		"/data/user/10/" + PackageName + "/files":       "/data/user/10/" + PackageName + "/files",
		"/data/user/0/" + PackageName + "/files/":       "/data/user/0/" + PackageName + "/files",
		"/data/data/" + PackageName + "/files":          "/data/data/" + PackageName + "/files",
		"/home/someone":                                 fallback,
		"":                                              fallback,
		"data/user/10/" + PackageName + "/files":        fallback,
		"/data/user/10/" + PackageName + "/cache":       fallback,
		"/data/user/10/other." + PackageName + "/files": fallback,
	}
	for home, want := range own {
		t.Setenv("HOME", home)
		if got := stateDir(); got != want {
			t.Errorf("HOME=%q: StateDir %q, want %q", home, got, want)
		}
	}
}
