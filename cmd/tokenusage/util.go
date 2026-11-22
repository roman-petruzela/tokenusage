package main

import (
	"encoding/json"

	"github.com/roman-petruzela/tokenusage/internal/tui"
)

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func runTUI() error { return tui.Run(loadCfg(), version) }
