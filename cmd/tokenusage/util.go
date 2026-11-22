package main

import "encoding/json"

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// runTUI is replaced by the real dashboard in the tui commit.
func runTUI() error { return nil }
