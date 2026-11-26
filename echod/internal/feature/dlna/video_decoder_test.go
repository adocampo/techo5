//go:build !dot && !panel

package dlna

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// Videos are offered only while the decoder is there: take techo5-ffmpeg away and the sink protocols
// lose video again. The panel stands this out — its decoder is Android's own and cannot be taken away.
func TestVideoIsOfferedOnlyWhileTheDecoderIs(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	exe, _ := os.Executable()
	t.Setenv("TECHO5_FFMPEG", exe)
	_ = config.Set().Video().On(true)
	_ = config.Set().Video().DLNA(true)
	f := &Feature{}
	f.r.f = f
	protocols := func() string {
		_, body := call(t, f, "ConnectionManager", "GetProtocolInfo", "")
		return body
	}
	if p := protocols(); !strings.Contains(p, "video/mp4") {
		t.Errorf("decoder there: %s", p)
	}
	t.Setenv("TECHO5_FFMPEG", filepath.Join(t.TempDir(), "none"))
	if p := protocols(); strings.Contains(p, "video/") {
		t.Errorf("no decoder: %s", p)
	}
}
