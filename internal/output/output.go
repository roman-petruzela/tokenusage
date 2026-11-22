// Package output renders Usage as JSON, a Waybar module or a one-line status.
package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/roman-petruzela/tokenusage/internal/model"
)

func JSON(u model.Usage) ([]byte, error) { return json.MarshalIndent(u, "", "  ") }

// Class returns ok|warn|crit|error from status and the highest utilization.
func Class(u model.Usage, warn, crit float64) string {
	if u.Status == model.StatusAuthExpired || u.Status == model.StatusNetworkError {
		return "error"
	}
	var hi float64
	for _, w := range windows(u) {
		if w.w != nil && w.w.Utilization > hi {
			hi = w.w.Utilization
		}
	}
	switch {
	case hi >= crit:
		return "crit"
	case hi >= warn:
		return "warn"
	}
	return "ok"
}

type named struct {
	key, label string
	w          *model.Window
}

func windows(u model.Usage) []named {
	return []named{
		{"five_hour", "5h", u.Limits.FiveHour},
		{"seven_day", "7d", u.Limits.SevenDay},
		{"seven_day_opus", "7d Opus", u.Limits.SevenDayOpus},
		{"seven_day_sonnet", "7d Sonnet", u.Limits.SevenDaySonnet},
	}
}

func pct(w *model.Window) string {
	if w == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.0f", w.Utilization)
}

// Render substitutes {five_hour} etc. in the template.
func Render(tmpl string, u model.Usage) string {
	var kv []string
	for _, n := range windows(u) {
		kv = append(kv, "{"+n.key+"}", pct(n.w))
	}
	return strings.NewReplacer(kv...).Replace(tmpl)
}

// HumanDuration formats seconds as "2h13m" / "5d3h" / "12m".
func HumanDuration(sec int64) string {
	d := time.Duration(sec) * time.Second
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// WindowLine is "42% · resets in 2h13m (14:30)" or "n/a".
func WindowLine(w *model.Window) string {
	if w == nil {
		return "n/a"
	}
	s := pct(w) + "%"
	if w.ResetsAt != nil {
		s += fmt.Sprintf(" · resets in %s (%s)", HumanDuration(w.ResetsInSeconds), w.ResetsAt.Local().Format("15:04"))
	}
	return s
}

type WaybarOut struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip"`
	Class      string `json:"class"`
	Percentage int    `json:"percentage"`
	Alt        string `json:"alt"`
}

func Waybar(u model.Usage, tmpl string, warn, crit float64) WaybarOut {
	cls := Class(u, warn, crit)
	out := WaybarOut{Text: Render(tmpl, u), Class: cls, Alt: u.Status}
	if u.Limits.FiveHour != nil {
		out.Percentage = int(u.Limits.FiveHour.Utilization)
	}
	var b strings.Builder
	for _, n := range windows(u) {
		if n.w != nil {
			fmt.Fprintf(&b, "%s: %s\n", n.label, WindowLine(n.w))
		}
	}
	fmt.Fprintf(&b, "Tokens: 5h %s · today %s · week %s", Short(u.Tokens.Block5h.Total), Short(u.Tokens.Today.Total), Short(u.Tokens.Week.Total))
	if u.Plan.Subscription != "" {
		fmt.Fprintf(&b, "\nPlan: %s", u.Plan.Subscription)
	}
	if u.Error != "" {
		fmt.Fprintf(&b, "\n⚠ %s", u.Error)
	}
	if u.Status == model.StatusAuthExpired || u.Status == model.StatusNetworkError {
		out.Text = "⚠ " + out.Text
	}
	out.Tooltip = b.String()
	return out
}

// Short abbreviates token counts: 1.2k, 3.4M.
func Short(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

func Status(u model.Usage) string {
	s := fmt.Sprintf("[%s] 5h %s · 7d %s", u.Status, pct(u.Limits.FiveHour)+"%", pct(u.Limits.SevenDay)+"%")
	if u.Error != "" {
		s += " — " + u.Error
	}
	return s
}
