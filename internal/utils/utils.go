package utils

import (
	"encoding/json"
	"fmt"
	"regexp"

	"phantom/internal/ui/components/styles" // Corrected import path
)

// FormatBytes converts bytes to a human-readable string.
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// PrettyPrintJSON formats and colorizes a JSON string.
func PrettyPrintJSON(input string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return input // Not valid JSON, return as is
	}
	pretty, _ := json.MarshalIndent(data, "", "  ")

	s := string(pretty)
	s = regexp.MustCompile(`"([^"]+)":`).ReplaceAllString(s, styles.JSONKeyStyle.Render(`"$1"`)+":")
	s = regexp.MustCompile(`: "([^"]*)"`).ReplaceAllString(s, `: `+styles.JSONStringStyle.Render(`"$1"`))
	s = regexp.MustCompile(`: ([0-9\.]+)`).ReplaceAllString(s, `: `+styles.JSONNumberStyle.Render(`$1`))
	s = regexp.MustCompile(`: (true|false)`).ReplaceAllString(s, `: `+styles.JSONBoolStyle.Render(`$1`))
	s = regexp.MustCompile(`: null`).ReplaceAllString(s, `: `+styles.JSONNullStyle.Render(`null`))
	return s
}
