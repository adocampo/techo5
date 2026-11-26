//go:build intercom

package layout

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TECHO5 Intercom: an Android phone or tablet running echod as a plain intercom — no wake word, no
// assistant, no screen of echod's own. It is the panel's build (tag panel) with the tag intercom
// added: the same app plumbing over local sockets (panelsock: mic, speaker, control), but the app
// serves no screen socket and the components that would want one — the display, the camera, the
// Bluetooth proxy, the home dashboard — never register. The control line carries calls and peers
// instead of pixels (feature/phone/ctl_panel.go).
const (
	// PackageName is the Android app echod runs inside. Its files directory is the only place the
	// daemon may write.
	PackageName = "app.techo5.intercom"

	BinaryName  = "techo5"
	ServiceName = "techo5"

	StockLabel = "u:object_r:system_file:s0"

	StartAnimation = ""
	StopAnimation  = ""

	FirewallHook = ""

	// LogTag is the daemon's logcat tag: `adb logcat -s techo5`.
	LogTag = "techo5"

	Manufacturer = "TECHO5"

	// DefaultName is the fallback display name when a device has none recorded.
	DefaultName = "TECHO5 Intercom"
)

// StateDir is the app's files directory, which the app starts echod with as HOME and as its working
// directory. It is found rather than compiled in: Android gives each user of the phone its own copy
// (/data/user/<user>/app.techo5.intercom/files). The first user's is the fallback, for a daemon
// started by hand.
var StateDir = stateDir()

var (
	// AndroidDir is where an in-place update would put the binary. The app ships the daemon in its
	// own APK and updates come with the app, so nothing is ever written here.
	AndroidDir = StateDir + "/bin"
	Service    = AndroidDir + "/" + BinaryName

	LogPath = StateDir + "/techo5.log"
	BootLog = StateDir + "/boot.log"
)

func stateDir() string {
	for _, d := range []string{os.Getenv("HOME"), workingDir()} {
		if filepath.IsAbs(d) && strings.HasSuffix(filepath.Clean(d), "/"+PackageName+"/files") {
			return filepath.Clean(d)
		}
	}
	return "/data/user/0/" + PackageName + "/files"
}

func workingDir() string {
	d, _ := os.Getwd()
	return d
}

var AnimationScripts = []string{}

// Board is the Android device name and Model what the phone calls itself, read once at start from
// the system properties an app may read.
var (
	Board = getprop("ro.product.device", "android")
	Model = "TECHO5 Intercom (" + getprop("ro.product.model", "Android") + ")"
)

// ZoneDB is Android's packed zone database ("tzdata"): the APEX copy that system updates keep
// current on Android 10 and later, an update dropped in data on older ones, or the one the system
// image shipped with. Empty when there is none.
var ZoneDB = firstFile(
	"/apex/com.android.tzdata/etc/tz/tzdata",
	"/data/misc/zoneinfo/current/tzdata",
	"/system/usr/share/zoneinfo/tzdata",
)

func firstFile(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// The zone Home Assistant gave, which feature/timezone keeps at StateDir/localtime. Go never reads
// a local zone on Android (time.Local is always UTC there), so it is loaded here, in the package
// everything else imports first, before anything has read the clock.
//
// ZONEINFO is pointed at Android's zone database first, for the zones named elsewhere: Go only
// knows the places older Androids kept it, and Android 10 and later keep it in an APEX.
func init() {
	if ZoneDB != "" && os.Getenv("ZONEINFO") == "" {
		os.Setenv("ZONEINFO", ZoneDB)
	}
	b, err := os.ReadFile(StateDir + "/localtime")
	if err != nil {
		return
	}
	if loc, err := time.LoadLocationFromTZData("Local", b); err == nil {
		time.Local = loc
	}
}

// An intercom is none of the Echo boards, and not a panel either: nothing that asks "is this a
// panel" should answer yes for a device with no screen to draw on.
func Checkers() bool { return false }

func Crown() bool { return false }

func Show8() bool { return false }

func Athena() bool { return false }

// Intercom is what the components ask: an intercom takes no wake word, shows no screen of its own,
// and points no camera at the room.
func Intercom() bool { return true }

func getprop(name, fallback string) string {
	out, err := exec.Command("/system/bin/getprop", name).Output()
	if v := strings.TrimSpace(string(out)); err == nil && v != "" {
		return v
	}
	return fallback
}
