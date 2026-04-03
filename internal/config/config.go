package config

import (
	"log"
	"os"
	"path/filepath"

	"phantom/internal/ui/tabs/http"
	"phantom/internal/ui/tabs/logs"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	lua "github.com/yuin/gopher-lua"
)

// ConfigLoadedMsg is sent when the Lua configuration is successfully loaded.
type ConfigLoadedMsg struct {
	Templates   []list.Item
	Environment map[string]string
	LogFile     string
	LogSources  []logs.SourceConfig
}

// LoadConfig reads and parses the config.lua file.
func LoadConfig() tea.Cmd {
	return func() tea.Msg {
		configPath := findConfigPath()
		if configPath == "" {
			log.Println("config.lua not found. Using defaults.")
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}, LogFile: "debug.log", LogSources: nil}
		}

		L := lua.NewState()
		defer L.Close()

		if err := L.DoFile(configPath); err != nil {
			log.Printf("could not load %s: %v. Using defaults.", configPath, err)
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}, LogFile: "debug.log", LogSources: nil}
		}

		configTable, ok := L.GetGlobal("Config").(*lua.LTable)
		if !ok {
			log.Println("'Config' table not found. Using defaults.")
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}, LogFile: "debug.log", LogSources: nil}
		}

		httpTable, ok := configTable.RawGetString("http").(*lua.LTable)
		if !ok {
			log.Println("'http' table not found in Config. Using defaults.")
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}, LogFile: "debug.log", LogSources: nil}
		}

		// Load templates
		var templates []list.Item
		templatesTable, ok := httpTable.RawGetString("templates").(*lua.LTable)
		if ok {
			templatesTable.ForEach(func(_, val lua.LValue) {
				t, ok := val.(*lua.LTable)
				if !ok {
					return
				}
				templates = append(templates, http.RequestItem{
					Name:    t.RawGetString("name").String(),
					Method:  t.RawGetString("method").String(),
					URL:     t.RawGetString("url").String(),
					Headers: t.RawGetString("headers").String(),
					Body:    t.RawGetString("body").String(),
				})
			})
		}

		// Load environment
		environment := make(map[string]string)
		envTable, ok := httpTable.RawGetString("environment").(*lua.LTable)
		if ok {
			envTable.ForEach(func(key, val lua.LValue) {
				environment[key.String()] = val.String()
			})
		}

		logFile := "debug.log"
		var logSources []logs.SourceConfig
		if logsTable, ok := configTable.RawGetString("logs").(*lua.LTable); ok {
			if value := logsTable.RawGetString("file"); value.Type() == lua.LTString {
				logFile = value.String()
			}
			if sourcesTable, ok := logsTable.RawGetString("sources").(*lua.LTable); ok {
				sourcesTable.ForEach(func(_, val lua.LValue) {
					row, ok := val.(*lua.LTable)
					if !ok {
						return
					}
					src := logs.SourceConfig{
						Name:    row.RawGetString("name").String(),
						Path:    row.RawGetString("path").String(),
						Unit:    row.RawGetString("unit").String(),
						Cmd:     row.RawGetString("cmd").String(),
						Color:   row.RawGetString("color").String(),
						Enabled: true,
					}
					if enabled := row.RawGetString("enabled"); enabled.Type() == lua.LTBool {
						src.Enabled = lua.LVAsBool(enabled)
					}
					t := row.RawGetString("type").String()
					switch t {
					case string(logs.SourceFile):
						src.Type = logs.SourceFile
					case string(logs.SourceJournaldUnit):
						src.Type = logs.SourceJournaldUnit
					case string(logs.SourceCommand):
						src.Type = logs.SourceCommand
					}
					logSources = append(logSources, src)
				})
			}
		}

		return ConfigLoadedMsg{Templates: templates, Environment: environment, LogFile: logFile, LogSources: logSources}
	}
}

func findConfigPath() string {
	candidates := []string{"config.lua"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "phantom", "config.lua"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
