package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/roman-petruzela/tokenusage/internal/collect"
	"github.com/roman-petruzela/tokenusage/internal/config"
	"github.com/roman-petruzela/tokenusage/internal/model"
	"github.com/roman-petruzela/tokenusage/internal/output"
)

var version = "dev"

const (
	exitAuth    = 2
	exitNetwork = 3
)

func main() {
	if err := root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func loadCfg() config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	return cfg
}

// exitFor maps a document status to the refresh exit code.
func exitFor(u model.Usage) int {
	switch u.Status {
	case model.StatusAuthExpired:
		return exitAuth
	case model.StatusNetworkError:
		return exitNetwork
	}
	return 0
}

func root() *cobra.Command {
	rc := &cobra.Command{
		Use:           "tokenusage",
		Short:         "Claude Pro/Max usage tracker (TUI + JSON for Waybar/Quickshell)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(*cobra.Command, []string) error { return runTUI() },
	}

	var cached bool
	var maxAge time.Duration
	load := func(c *cobra.Command) model.Usage {
		return collect.Load(c.Context(), loadCfg(), version, maxAge, cached)
	}
	addFlags := func(c *cobra.Command) {
		c.Flags().BoolVar(&cached, "cached", false, "never hit the network; serve the cache")
		c.Flags().DurationVar(&maxAge, "max-age", 60*time.Second, "refetch only if the cache is older than this")
	}

	jsonCmd := &cobra.Command{Use: "json", Short: "Print normalized usage JSON", RunE: func(c *cobra.Command, _ []string) error {
		b, err := output.JSON(load(c))
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}}
	addFlags(jsonCmd)

	waybarCmd := &cobra.Command{Use: "waybar", Short: "Print Waybar custom-module JSON", RunE: func(c *cobra.Command, _ []string) error {
		cfg := loadCfg()
		u := collect.Load(c.Context(), cfg, version, maxAge, cached)
		b, err := jsonMarshal(output.Waybar(u, cfg.WaybarText, cfg.WarnThreshold, cfg.CritThreshold))
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}}
	addFlags(waybarCmd)

	refreshCmd := &cobra.Command{Use: "refresh", Short: "Fetch and write the cache (exit 2 auth, 3 network)", Run: func(c *cobra.Command, _ []string) {
		u := collect.Refresh(c.Context(), loadCfg(), version)
		if u.Error != "" {
			fmt.Fprintln(os.Stderr, u.Error)
		}
		os.Exit(exitFor(u))
	}}

	statusCmd := &cobra.Command{Use: "status", Short: "One-line human status", Run: func(c *cobra.Command, _ []string) {
		u := load(c)
		fmt.Println(output.Status(u))
		os.Exit(exitFor(u))
	}}
	addFlags(statusCmd)

	cfgCmd := &cobra.Command{Use: "config", Short: "Config helpers"}
	cfgCmd.AddCommand(
		&cobra.Command{Use: "path", Short: "Print config path", Run: func(*cobra.Command, []string) { fmt.Println(config.Path()) }},
		&cobra.Command{Use: "init", Short: "Write default config", RunE: func(*cobra.Command, []string) error {
			p, err := config.Init()
			if err == nil {
				fmt.Println("wrote", p)
			}
			return err
		}},
	)

	rc.AddCommand(
		&cobra.Command{Use: "tui", Short: "Interactive dashboard", RunE: func(*cobra.Command, []string) error { return runTUI() }},
		jsonCmd, waybarCmd, refreshCmd, statusCmd, cfgCmd,
		&cobra.Command{Use: "version", Short: "Print version", Run: func(*cobra.Command, []string) { fmt.Println(version) }},
	)
	rc.SetContext(context.Background())
	return rc
}
