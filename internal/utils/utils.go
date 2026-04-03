package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

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

// Yank copies text to clipboard with fallback order:
// OSC52 -> pbcopy/xclip/xsel -> /tmp/phantom_yank.
func Yank(text string) (method string, detail string) {
	if text == "" {
		return "noop", "empty"
	}

	if err := yankOSC52(text); err == nil {
		return "osc52", preview(text, 60)
	}

	if _, err := exec.LookPath("pbcopy"); err == nil {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return "pbcopy", preview(text, 60)
		}
	}

	if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return "xclip", preview(text, 60)
		}
	}

	if _, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return "xsel", preview(text, 60)
		}
	}

	const fallbackPath = "/tmp/phantom_yank"
	if err := os.WriteFile(fallbackPath, []byte(text), 0o600); err == nil {
		return "file", fallbackPath
	}

	return "failed", "clipboard unavailable"
}

func yankOSC52(text string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer tty.Close()

	enc := base64.StdEncoding.EncodeToString([]byte(text))
	seq := "\x1b]52;c;" + enc + "\x07"
	_, err = tty.Write([]byte(seq))
	return err
}

func preview(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if n <= 0 || len(s) <= n {
		return s
	}
	var b bytes.Buffer
	b.WriteString(s[:n-3])
	b.WriteString("...")
	return b.String()
}
