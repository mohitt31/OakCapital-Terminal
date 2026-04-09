package simbot

import (
	"strings"
	"time"
)

// ParseEvalInterval converts UI interval labels (e.g. "1m") into durations.
// If the label is empty/unknown it returns fallback.
func ParseEvalInterval(label string, fallback time.Duration) time.Duration {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "1s":
		return 1 * time.Second
	case "5s":
		return 5 * time.Second
	case "15s":
		return 15 * time.Second
	case "30s":
		return 30 * time.Second
	case "1m":
		return 1 * time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return 1 * time.Hour
	default:
		return fallback
	}
}

// FormatEvalInterval converts duration to a compact label for logs.
func FormatEvalInterval(d time.Duration) string {
	switch d {
	case 1 * time.Second:
		return "1s"
	case 5 * time.Second:
		return "5s"
	case 15 * time.Second:
		return "15s"
	case 30 * time.Second:
		return "30s"
	case 1 * time.Minute:
		return "1m"
	case 3 * time.Minute:
		return "3m"
	case 5 * time.Minute:
		return "5m"
	case 15 * time.Minute:
		return "15m"
	case 30 * time.Minute:
		return "30m"
	case 1 * time.Hour:
		return "1h"
	default:
		return d.String()
	}
}
