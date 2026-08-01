// Package chrono provides helpers for formatting times and durations.
package chrono

import (
	"fmt"
	"time"
)

func localTime(t time.Time) time.Time {
	return t.In(time.Local)
}

func RFC3339Time(t time.Time) string {
	return t.Format(time.RFC3339)
}

func HumanTime(t time.Time) string {
	t = localTime(t)

	if t.Second() == 0 {
		return t.Format("Mon, Jan 02, 15:04")
	}
	return t.Format("Mon, Jan 02, 15:04:05")
}

var months = []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
var days = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func formatDMHM(t time.Time) string {
	return fmt.Sprintf("%s %02d %02d:%02d", months[t.Month()], t.Day(), t.Hour(), t.Minute())
}

func formatDHM(t time.Time) string {
	return fmt.Sprintf("%s %02d:%02d", days[t.Weekday()], t.Hour(), t.Minute())
}

func formatHMS(d time.Duration) string {
	d = d.Round(time.Second)

	if d <= 0 {
		return "00:00:00"
	}

	hh := d / (time.Hour)
	d = d % (time.Hour)

	mm := d / (time.Minute)
	d = d % (time.Minute)

	ss := d / (time.Second)

	return fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)
}

func UnlockTime(t time.Time) string {
	t = localTime(t)
	now := localTime(time.Now())

	diff := t.Sub(now)
	if diff <= 0 {
		return ""
	}

	switch {
	case diff > 7*24*time.Hour:
		// More than a week away
		return formatDMHM(t)

	case t.Year() != now.Year() || t.Month() != now.Month() || t.Day() != now.Day():
		// Within a week but not today
		return formatDHM(t)

	default:
		// Same day
		return formatHMS(diff)
	}
}

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	dd := d / (24 * time.Hour)
	d = d % (24 * time.Hour)

	hh := d / (time.Hour)
	d = d % (time.Hour)

	mm := d / (time.Minute)
	d = d % (time.Minute)

	ss := d / (time.Second)

	switch {
	case dd > 0:
		return fmt.Sprintf("%dd %02dh %02dm %02ds", dd, hh, mm, ss)

	case hh > 0:
		return fmt.Sprintf("%dh %02dm %02ds", hh, mm, ss)

	case mm > 0:
		return fmt.Sprintf("%dm %02ds", mm, ss)

	default:
		return fmt.Sprintf("%ds", ss)
	}
}
