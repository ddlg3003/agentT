package greennode

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// configFileName is the optional JSON config file read from the working
// directory, mirroring the Python SDK's .greennode.json.
const configFileName = ".greennode.json"

var (
	configOnce  sync.Once
	configCache map[string]any
)

func loadConfigFile() map[string]any {
	configOnce.Do(func() {
		configCache = map[string]any{}
		data, err := os.ReadFile(configFileName)
		if err != nil {
			return // file is optional
		}
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) == nil {
			configCache = parsed
		}
	})
	return configCache
}

// GetConfigValue resolves a configuration value with the priority:
// environment variable -> .greennode.json -> default.
//
// For the config file it tries the key lowercased with any leading
// "GREENNODE_" prefix stripped (e.g. GREENNODE_CLIENT_ID -> client_id), then
// the raw key. This matches the Python SDK's resolution rules.
func GetConfigValue(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	cfg := loadConfigFile()
	if len(cfg) > 0 {
		configKey := strings.ToLower(key)
		configKey = strings.TrimPrefix(configKey, "greennode_")
		if v, ok := cfg[configKey]; ok {
			if s := stringify(v); s != "" {
				return s
			}
		}
		if v, ok := cfg[key]; ok {
			if s := stringify(v); s != "" {
				return s
			}
		}
	}
	return def
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
