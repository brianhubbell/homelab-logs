//go:build darwin

package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func installServiceUnit(name, plistPath, repoDir string) error {
	src := filepath.Join(repoDir, plistPath)
	dst := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", name)

	cpCmd := exec.Command("sudo", "cp", src, dst)
	out, err := cpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy plist: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func enableAndStartService(name string) error {
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", name)
	cmd := exec.Command("sudo", "launchctl", "load", "-w", plistPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("load %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func stopService(name string) error {
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", name)
	cmd := exec.Command("sudo", "launchctl", "unload", plistPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unload %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func restartService(name string) error {
	label := "system/com." + name
	cmd := exec.Command("sudo", "launchctl", "kickstart", "-k", label)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func isServiceRunning(name string) bool {
	cmd := exec.Command("launchctl", "list")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, name) {
			return true
		}
	}
	return false
}
