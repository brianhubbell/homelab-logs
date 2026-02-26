package goutils

import "strings"

// StrToBool converts a string to a boolean. Returns true for "true", "1", "yes"
// (case-insensitive, trimmed). All other values return false.
func StrToBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}
