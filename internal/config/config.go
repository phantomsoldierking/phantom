package config

import (
	"log"

	"phantom/internal/ui/tabs/http"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	lua "github.com/yuin/gopher-lua"
)

// ConfigLoadedMsg is sent when the Lua configuration is successfully loaded.
type ConfigLoadedMsg struct {
	Templates   []list.Item
	Environment map[string]string
}

// LoadConfig reads and parses the config.lua file.
func LoadConfig() tea.Cmd {
	return func() tea.Msg {
		L := lua.NewState()
		defer L.Close()

		if err := L.DoFile("config.lua"); err != nil {
			log.Printf("could not load config.lua: %v. Using defaults.", err)
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}}
		}

		configTable, ok := L.GetGlobal("Config").(*lua.LTable)
		if !ok {
			log.Println("'Config' table not found in config.lua. Using defaults.")
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}}
		}

		httpTable, ok := configTable.RawGetString("http").(*lua.LTable)
		if !ok {
			log.Println("'http' table not found in Config. Using defaults.")
			return ConfigLoadedMsg{Templates: []list.Item{}, Environment: map[string]string{}}
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

		return ConfigLoadedMsg{Templates: templates, Environment: environment}
	}
}
