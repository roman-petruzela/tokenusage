// Package tui is the Bubble Tea dashboard.
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/roman-petruzela/tokenusage/internal/collect"
	"github.com/roman-petruzela/tokenusage/internal/config"
	"github.com/roman-petruzela/tokenusage/internal/model"
	"github.com/roman-petruzela/tokenusage/internal/output"
)

type keyMap struct{ Refresh, Quit, Help key.Binding }

func (k keyMap) ShortHelp() []key.Binding  { return []key.Binding{k.Refresh, k.Help, k.Quit} }
func (k keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Refresh, k.Help, k.Quit}} }

var keys = keyMap{
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

type (
	loadedMsg model.Usage
	tickMsg   time.Time
)

type Model struct {
	cfg      config.Config
	version  string
	usage    model.Usage
	have     bool
	loading  bool
	showHelp bool
	spin     spinner.Model
	help     help.Model
	bar      progress.Model
	now      time.Time
}

func New(cfg config.Config, version string) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	bar := progress.New(progress.WithoutPercentage(), progress.WithWidth(30), progress.WithSolidFill("2"))
	return Model{cfg: cfg, version: version, loading: true, spin: sp, help: help.New(), bar: bar, now: time.Now()}
}

// Run starts the dashboard.
func Run(cfg config.Config, version string) error {
	_, err := tea.NewProgram(New(cfg, version), tea.WithAltScreen()).Run()
	return err
}

func (m Model) fetch() tea.Cmd {
	return func() tea.Msg {
		// Dashboard honors the refresh interval via tick; cached data within 5s is reused.
		return loadedMsg(collect.Load(context.Background(), m.cfg, m.version, 5*time.Second, false))
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.cfg.RefreshInterval.D(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetch(), m.tick(), m.spin.Tick, tea.Tick(time.Second, func(t time.Time) tea.Msg { return t }))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Refresh):
			m.loading = true
			return m, m.fetch()
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
		}
	case loadedMsg:
		m.usage, m.have, m.loading = model.Usage(msg), true, false
	case tickMsg:
		m.loading = true
		return m, tea.Batch(m.fetch(), m.tick())
	case time.Time: // 1s clock tick keeps "resets in" and "updated ago" live
		m.now = msg
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return t })
	case spinner.TickMsg:
		var c tea.Cmd
		m.spin, c = m.spin.Update(msg)
		return m, c
	}
	return m, nil
}

func (m Model) color(pct float64) string {
	switch {
	case pct >= m.cfg.CritThreshold:
		return "1"
	case pct >= m.cfg.WarnThreshold:
		return "3"
	}
	return "2"
}

func (m Model) limitRow(label string, w *model.Window) string {
	lbl := lipgloss.NewStyle().Width(11).Render(label)
	if w == nil {
		return lbl + " n/a"
	}
	b := m.bar
	b.FullColor = m.color(w.Utilization)
	line := fmt.Sprintf("%s %s %3.0f%%", lbl, b.ViewAs(w.Utilization/100), w.Utilization)
	if w.ResetsAt != nil {
		left := max(int64(w.ResetsAt.Sub(m.now).Seconds()), 0)
		line += fmt.Sprintf("  resets in %s (%s)", output.HumanDuration(left), w.ResetsAt.Local().Format("15:04"))
	}
	return line
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true)
	var b strings.Builder
	b.WriteString(title.Render("tokenusage") + " " + m.version + "\n\n")
	if !m.have {
		return b.String() + m.spin.View() + " loading…\n"
	}
	u := m.usage
	if u.Plan.Subscription != "" {
		fmt.Fprintf(&b, "Plan: %s %s\n\n", u.Plan.Subscription, u.Plan.Tier)
	}
	b.WriteString(m.limitRow("5h", u.Limits.FiveHour) + "\n")
	b.WriteString(m.limitRow("7d", u.Limits.SevenDay) + "\n")
	b.WriteString(m.limitRow("7d Opus", u.Limits.SevenDayOpus) + "\n")
	b.WriteString(m.limitRow("7d Sonnet", u.Limits.SevenDaySonnet) + "\n\n")

	b.WriteString(title.Render("Tokens (local Claude Code transcripts)") + "\n")
	for _, p := range []struct {
		name string
		c    model.Counts
	}{{"5h block", u.Tokens.Block5h}, {"Today", u.Tokens.Today}, {"7 days", u.Tokens.Week}, {"Month", u.Tokens.Month}} {
		b.WriteString(countsBlock(p.name, p.c))
	}

	status := fmt.Sprintf("status: %s · updated %s ago", u.Status, output.HumanDuration(int64(m.now.Sub(u.UpdatedAt).Seconds())))
	if m.loading {
		status = m.spin.View() + " refreshing · " + status
	}
	style := lipgloss.NewStyle().Faint(true)
	if u.Error != "" {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		status += " — " + u.Error
	}
	b.WriteString("\n" + style.Render(status) + "\n")
	m.help.ShowAll = m.showHelp
	b.WriteString(m.help.View(keys))
	return b.String()
}

func countsBlock(name string, c model.Counts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %-9s total %-7s in %-7s out %-7s cache-w %-7s cache-r %s\n", name,
		output.Short(c.Total), output.Short(c.Input), output.Short(c.Output), output.Short(c.CacheCreation), output.Short(c.CacheRead))
	models := make([]string, 0, len(c.ByModel))
	for k := range c.ByModel {
		models = append(models, k)
	}
	sort.Strings(models)
	for _, k := range models {
		mc := c.ByModel[k]
		fmt.Fprintf(&b, "      %-28s %-7s in %-7s out %-7s\n", k, output.Short(mc.Total), output.Short(mc.Input), output.Short(mc.Output))
	}
	return b.String()
}
