//go:build !dot && !spot

package display

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/alarm"
	"github.com/HuskerMinion/techo5/echod/internal/feature/home"
	"github.com/HuskerMinion/techo5/echod/internal/feature/timer"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// Every clock style draws on both panels, alone, with a timer running, with the glance strip, with
// the music strip and with the chips over it; with SHOW_PREVIEW set, each is written there to look at.
func TestShowClockStylesDraw(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	previewFont(t)
	t.Cleanup(func() {
		_ = config.Set().Screen().ClockStyle("")
		_ = config.Set().Screen().GlanceWithMusic("")
	})
	// Chips while music plays: Above the strip, for the scene with both; the others have one or neither.
	if err := config.Set().Screen().GlanceWithMusic("above"); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 16, 14, 7, 38, 0, time.Local)
	sky := home.Weather{Condition: "partlycloudy", Temp: "72°"}
	var week []hass.Day
	for i, c := range []string{"partlycloudy", "rainy", "sunny", "cloudy", "snowy"} {
		week = append(week, hass.Day{When: at.AddDate(0, 0, i), Condition: c, High: float64(72 - 3*i), Low: float64(51 - 2*i)})
	}
	facts := styleFacts{
		rise: time.Date(2026, 9, 16, 6, 48, 0, 0, time.Local), set: time.Date(2026, 9, 16, 19, 5, 0, 0, time.Local), sunOK: true,
		days: week,
		next: []hass.Event{
			{Summary: "Dentist", Start: at.Add(83 * time.Minute), End: at.Add(143 * time.Minute)},
			{Summary: "Soccer practice", Start: at.Add(233 * time.Minute), End: at.Add(293 * time.Minute)},
			{Summary: "Trash day", Start: time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local), End: time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local), AllDay: true},
		},
		places: worldPlaces(nil),
	}
	named := facts
	named.named = "Binary"
	later := facts
	later.next = append([]hass.Event{{Summary: "Doctor's appointment", Start: time.Date(2026, 9, 17, 9, 45, 0, 0, time.Local), End: time.Date(2026, 9, 17, 10, 45, 0, 0, time.Local)},
		{Summary: "Still going", Start: at.Add(-30 * time.Minute), End: at.Add(time.Hour)}}, facts.next[:1]...)
	running := []timer.Countdown{{Name: "Pasta", Left: 4*time.Minute + 32*time.Second, Total: 10 * time.Minute, Active: true}}
	chips := []home.Chip{{Icon: "mdi:door", Text: "Back door open"}, {Icon: "mdi:thermometer", Text: "Upstairs 74°"}}
	scenes := map[string]scene{
		"":        {now: at, phase: "idle", weather: sky, style: facts},
		"-named":  {now: at, phase: "idle", weather: sky, style: named},
		"-24h":    {now: at, phase: "idle", weather: sky, style: facts},
		"-timer":  {now: at, phase: "idle", weather: sky, style: facts, timers: running},
		"-glance": {now: at, phase: "idle", weather: sky, style: facts, glance: chips},
		"-strip": {now: at, phase: "idle", weather: sky, style: facts, strip: true, playing: true,
			radio: home.Radio{Now: "KXYZ 101.1", Title: "Take It Easy", Artist: "Eagles"}},
		"-strip-glance": {now: at, phase: "idle", weather: sky, style: facts, glance: chips, strip: true, playing: true,
			radio: home.Radio{Now: "KXYZ 101.1", Title: "Take It Easy", Artist: "Eagles"}},
		"-dawn": {now: time.Date(2026, 9, 16, 5, 30, 0, 0, time.Local), phase: "idle", weather: sky, style: facts},
		// The next alarm makes the date line longer; an event tomorrow at a time makes the when longer.
		"-alarm": {now: at, phase: "idle", weather: sky, style: later, alarms: alarm.View{Next: &alarm.Upcoming{At: at.Add(16 * time.Hour)}}},
		"-strip-timer": {now: at, phase: "idle", weather: sky, style: facts, timers: running, strip: true, playing: true,
			radio: home.Radio{Now: "KXYZ 101.1", Title: "Take It Easy", Artist: "Eagles"}},
	}
	dir := os.Getenv("SHOW_PREVIEW")
	for _, st := range clockStyles {
		if err := config.Set().Screen().ClockStyle(st.value); err != nil {
			t.Fatal(err)
		}
		for suffix, s := range scenes {
			clock24.Store(suffix == "-24h")
			for _, panel := range []struct {
				name       string
				wide, high int
			}{{"", showWide, showHigh}, {"-show8", show8Wide, show8High}, {"-skylight", skylightWide, skylightHigh}} {
				img := image.NewRGBA(image.Rect(0, 0, panel.wide, panel.high))
				newRenderer(img).draw(s)
				if dir == "" {
					continue
				}
				name := "style-" + st.label + suffix + panel.name + ".png"
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, img); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
		}
	}
	clock24.Store(false)
}

func TestClockWords(t *testing.T) {
	for _, c := range []struct {
		h, m                      int
		intro, lead, hour, period string
	}{
		{14, 7, "it's", "seven past", "two", "in the afternoon"},
		{14, 0, "it's", "", "two o'clock", "in the afternoon"},
		{14, 15, "it's", "quarter past", "two", "in the afternoon"},
		{14, 30, "it's", "half past", "two", "in the afternoon"},
		{14, 45, "it's", "quarter to", "three", "in the afternoon"},
		{14, 37, "it's", "twenty-three to", "three", "in the afternoon"},
		{9, 1, "it's", "a minute past", "nine", "in the morning"},
		{11, 59, "it's", "a minute to", "twelve", "in the morning"},
		{12, 0, "it's", "", "noon", ""},
		{0, 0, "it's", "", "midnight", ""},
		{23, 50, "it's", "ten to", "twelve", "at night"},
		{19, 20, "it's", "twenty past", "seven", "in the evening"},
		{4, 45, "it's", "quarter to", "five", "in the morning"}, // the part of the day is the hour said
		{17, 45, "it's", "quarter to", "six", "in the evening"},
	} {
		intro, lead, hour, period := clockWords(time.Date(2026, 9, 16, c.h, c.m, 0, 0, time.Local))
		if intro != c.intro || lead != c.lead || hour != c.hour || period != c.period {
			t.Errorf("%02d:%02d = %q %q %q %q, want %q %q %q %q", c.h, c.m, intro, lead, hour, period, c.intro, c.lead, c.hour, c.period)
		}
	}
}

// The Spanish clock keeps the English layout - intro, minutes, hour in colour with its part of the
// day beside it - but says the hour first: "son las / dos / de la tarde / y media". It is exact to
// the minute, and says the minutes with menos only where Spanish does - menos veinte, menos cuarto,
// menos diez, menos cinco - and with y everywhere else: "y treinta y siete", never "menos
// veintitrés", and "y cincuenta y nueve", never "menos uno". It counts the day by the sun: with no
// place set the madrugada ends at six and the noche begins at eight. Mediodía and medianoche are the
// exact hours only; the twelve hours go without a part of the day.
func TestClockWordsSpanish(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	t.Cleanup(func() { _ = config.Set().Screen().Language("") })
	if err := config.Set().Screen().Language("es"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		h, m                      int
		intro, lead, hour, period string
	}{
		{14, 7, "son las", "y siete", "dos", "de la tarde"},
		{14, 0, "son las", "", "dos en punto", "de la tarde"},
		{14, 15, "son las", "y cuarto", "dos", "de la tarde"},
		{14, 30, "son las", "y media", "dos", "de la tarde"},
		{14, 37, "son las", "y treinta y siete", "dos", "de la tarde"}, // not "menos veintitrés"
		{14, 40, "son las", "menos veinte", "tres", "de la tarde"},
		{14, 43, "son las", "y cuarenta y tres", "dos", "de la tarde"}, // not "menos diecisiete"
		{14, 45, "son las", "menos cuarto", "tres", "de la tarde"},
		{14, 47, "son las", "y cuarenta y siete", "dos", "de la tarde"},
		{14, 50, "son las", "menos diez", "tres", "de la tarde"},
		{14, 55, "son las", "menos cinco", "tres", "de la tarde"},
		{14, 56, "son las", "y cincuenta y seis", "dos", "de la tarde"}, // not "menos cuatro"
		{14, 58, "son las", "y cincuenta y ocho", "dos", "de la tarde"},
		{14, 59, "son las", "y cincuenta y nueve", "dos", "de la tarde"}, // not "menos uno"
		{13, 1, "es la", "y uno", "una", "de la tarde"},
		{1, 30, "es la", "y media", "una", "de la madrugada"},
		{0, 10, "son las", "y diez", "doce", "de la madrugada"},
		{4, 45, "son las", "menos cuarto", "cinco", "de la madrugada"},
		{6, 0, "son las", "", "seis en punto", "de la mañana"},
		{11, 45, "son las", "menos cuarto", "doce", "de la mañana"},
		{12, 0, "", "", "mediodía", ""},
		{12, 10, "son las", "y diez", "doce", ""},
		{12, 37, "son las", "y treinta y siete", "doce", ""},
		{12, 40, "es la", "menos veinte", "una", "de la tarde"},
		{12, 45, "es la", "menos cuarto", "una", "de la tarde"},
		{0, 0, "", "", "medianoche", ""},
		{19, 20, "son las", "y veinte", "siete", "de la tarde"},
		{20, 10, "son las", "y diez", "ocho", "de la noche"},
		{23, 50, "son las", "menos diez", "doce", "de la noche"},
	} {
		intro, lead, hour, period := clockWords(time.Date(2026, 9, 16, c.h, c.m, 0, 0, time.Local))
		if intro != c.intro || lead != c.lead || hour != c.hour || period != c.period {
			t.Errorf("%02d:%02d = %q %q %q %q, want %q %q %q %q", c.h, c.m, intro, lead, hour, period, c.intro, c.lead, c.hour, c.period)
		}
	}
}
