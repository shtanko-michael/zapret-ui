package main

import (
	"os"
	"strings"
)

// RUN_PROCESS_HIDDEN controls whether external helper processes (PowerShell/cmd/bat) are launched hidden.
//
// Default: true (hide).
// Override for debugging by setting env var RUN_PROCESS_HIDDEN=false (or 0/no/off).
var RUN_PROCESS_HIDDEN = false

func init() {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("RUN_PROCESS_HIDDEN")))
	if v == "" {
		return
	}
	switch v {
	case "0", "false", "no", "off":
		RUN_PROCESS_HIDDEN = false
	case "1", "true", "yes", "on":
		RUN_PROCESS_HIDDEN = true
	}
}
