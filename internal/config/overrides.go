package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	goutils "github.com/brianhubbell/go-utils"
)

const overridesFilename = "overrides.json"

// resolveDataDir returns the agent data directory, checking AGENT_DATA_DIR
// env var first, then /var/lib/homelab-agent, then $HOME/.config/homelab-agent.
func resolveDataDir() string {
	if dir := os.Getenv("AGENT_DATA_DIR"); dir != "" {
		return dir
	}
	return "/var/lib/homelab-agent"
}

// overridesPath returns the path to the overrides file.
func overridesPath() string {
	dir := resolveDataDir()
	if err := os.MkdirAll(dir, 0755); err == nil {
		return filepath.Join(dir, overridesFilename)
	}
	// Fallback to current working directory.
	return overridesFilename
}

// LoadOverrides reads persisted config overrides from disk.
// Returns an empty map if the file does not exist.
func LoadOverrides() (map[string]string, error) {
	path := overridesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var overrides map[string]string
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, err
	}
	if overrides == nil {
		overrides = make(map[string]string)
	}
	return overrides, nil
}

// SaveOverride persists a single config key-value pair to the overrides file.
// It reads the existing overrides, merges the new value, and writes back.
func SaveOverride(key, value string) error {
	overrides, err := LoadOverrides()
	if err != nil {
		goutils.Err("loading overrides for save", "error", err)
		overrides = make(map[string]string)
	}
	overrides[key] = value
	return writeOverrides(overrides)
}

func writeOverrides(overrides map[string]string) error {
	data, err := json.MarshalIndent(overrides, "", "  ")
	if err != nil {
		return err
	}
	path := overridesPath()
	return os.WriteFile(path, data, 0644)
}
