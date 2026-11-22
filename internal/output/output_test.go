package output

import (
	"testing"

	"github.com/roman-petruzela/tokenusage/internal/model"
)

func usage(status string, five, week float64) model.Usage {
	return model.Usage{Status: status, Limits: model.Limits{
		FiveHour: &model.Window{Utilization: five}, SevenDay: &model.Window{Utilization: week}}}
}

func TestClass(t *testing.T) {
	tests := []struct {
		name string
		u    model.Usage
		want string
	}{
		{"ok", usage("ok", 10, 20), "ok"},
		{"warn at threshold", usage("ok", 70, 0), "warn"},
		{"crit from week", usage("ok", 10, 95), "crit"},
		{"auth error", usage(model.StatusAuthExpired, 99, 99), "error"},
		{"network error", usage(model.StatusNetworkError, 0, 0), "error"},
		{"stale uses util", usage(model.StatusStale, 80, 0), "warn"},
		{"no limits", model.Usage{Status: "ok"}, "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Class(tt.u, 70, 90); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	u := usage("ok", 42.4, 7)
	tests := []struct{ tmpl, want string }{
		{"{five_hour}% · {seven_day}%", "42% · 7%"},
		{"{seven_day_opus}", "n/a"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		if got := Render(tt.tmpl, u); got != tt.want {
			t.Errorf("Render(%q) = %q, want %q", tt.tmpl, got, tt.want)
		}
	}
}

func TestWaybar(t *testing.T) {
	w := Waybar(usage("ok", 95, 1), "{five_hour}%", 70, 90)
	if w.Class != "crit" || w.Percentage != 95 || w.Text != "95%" || w.Tooltip == "" {
		t.Errorf("%+v", w)
	}
}
