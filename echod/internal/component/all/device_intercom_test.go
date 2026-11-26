//go:build intercom

package all

import "github.com/HuskerMinion/techo5/echod/internal/lib/panelscreen"

// No app answers in a test: the screen gives up on it at once rather than waiting for it at every
// restore of every setting (panelscreen.DialWait), which took this package past its ten minutes.
func init() { panelscreen.DialWait = 0 }

// notOnPanel is the panel's list (device_panel_test.go) and then some: an intercom registers no
// display, no camera, no Bluetooth proxy and no home dashboard, so everything those showed goes
// too. The panel's own reasons carry over — it is the same plumbing under the app.
var notOnPanel = []string{
	"update_channel", "airplay", "airplay_sets_volume", "spotify_connect",
	"video", "dlna_video", "video_state", "video_title", "video_error", "camera_live", "camera_sound",
	"bluetooth_audio", "bluetooth_disconnect", "bluetooth_forget", "bluetooth_forget_phones", "bluetooth_pairing",
	"bluetooth_phone_pairing", "bluetooth_reconnect", "bluetooth_reconnect_phone", "bluetooth_secure_connections",
	"bluetooth_sets_volume", "camera_web_access", "screen_night_warmth",
	// The wall clock bridge needs BlueZ over BR/EDR (wallclock.Here is false on Android).
	"wall_clock", "wall_clock_pair", "wall_clock_status",
	// A tablet's keys are Android's: the app hands the daemon no buttons (hardware/buttons.Here).
	"button_action", "button_mute", "button_volume_down", "button_volume_up",
	// The hotplug governor is the kernel's, which an app may not write (diag's coresHere).
	"min_cores",
	// A tablet's Wi-Fi is Android's (wifiquiet).
	"wifi_prefer_5ghz", "wifi_quiet_hours", "wifi_quiet_hours_end", "wifi_quiet_hours_start",
	// the Bluetooth proxy (not registered on an intercom).
	"bluetooth_proxy", "bluetooth_proxy_connections",
	// home's calendar popups (not registered on an intercom).
	"calendar_chip_size", "calendar_coming_up_refined", "calendar_countdowns", "calendar_dashboard_events",
	"calendar_day_timeline", "calendar_event_icons", "calendar_month_details", "calendar_next_up",
	"calendar_next_up_within", "calendar_opens_on", "calendar_popup_all_day", "calendar_popup_before",
	"calendar_popup_chime", "calendar_popup_sources_chosen", "calendar_popups", "calendar_sources_chosen",
	"calendar_weather", "calendar_week_starts_monday", "calendar_week_view",
	// the slideshow's photos (not registered on an intercom).
	"immich_favorites", "immich_from", "immich_person", "immich_to",
	"slideshow_caption", "slideshow_folder", "slideshow_interval", "slideshow_mode",
	"slideshow_pairs", "slideshow_screensaver_idle", "slideshow_screensaver_overlay", "slideshow_screensaver_position",
	"slideshow_screensaver_weather", "slideshow_shuffle", "slideshow_subfolders", "slideshow_weather_art",
	"slideshow_whole_photo",
	// the now playing page's (not registered on an intercom).
	"lyrics", "now_playing_album", "now_playing_artist", "now_playing_follows",
	"now_playing_source", "now_playing_state", "now_playing_title",
	// home's weather and radio (not registered on an intercom).
	"radar_source", "radio_artist", "radio_station", "radio_title",
	"weather_alerts", "weather_source",
	// the display's (not registered on an intercom).
	"screen", "screen_answer_time", "screen_at_night", "screen_auto_brightness",
	"screen_auto_brightness_dimmest", "screen_call_button", "screen_call_screen", "screen_camera_picture",
	"screen_camera_strips", "screen_camera_strips_shade", "screen_camera_time", "screen_clock_format",
	"screen_clock_position", "screen_clock_style", "screen_clock_style_swipe", "screen_clock_tap",
	"screen_date_color", "screen_dim_after_idle", "screen_font", "screen_glance_chip_size",
	"screen_glance_with_music", "screen_ignores_room_lights", "screen_language", "screen_light_theme",
	"screen_night_clock_style", "screen_night_end", "screen_night_hours", "screen_night_info",
	"screen_night_light_level", "screen_night_mode", "screen_night_start", "screen_night_tap_lifts",
	"screen_now_playing", "screen_power", "screen_skin", "screen_text_size",
	"screen_theme", "screen_theme_mode", "screen_timer_style", "screen_turn_style",
	"screen_volume_swipe", "screen_weather_animation", "screen_weather_position", "screen_weekend_night_end",
	"screen_weekend_night_start",
}

// onlyOnPanel is what a panel adds: Android can silence a tablet's microphone under the app.
var onlyOnPanel = []string{"microphone_silent"}
