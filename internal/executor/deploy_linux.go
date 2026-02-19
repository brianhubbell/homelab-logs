//go:build linux

package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func installServiceUnit(name, unitFilePath, repoDir string) error {
	src := filepath.Join(repoDir, unitFilePath)
	dst := fmt.Sprintf("/etc/systemd/system/%s.service", name)

	cpCmd := exec.Command("sudo", "cp", src, dst)
	out, err := cpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy unit file: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	reloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
	out, err = reloadCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("daemon-reload: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func enableAndStartService(name string) error {
	enableCmd := exec.Command("sudo", "systemctl", "enable", name)
	out, err := enableCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("enable %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}

	startCmd := exec.Command("sudo", "systemctl", "start", name)
	out, err = startCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}

	return nil
}

func stopService(name string) error {
	cmd := exec.Command("sudo", "systemctl", "stop", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func isServiceRunning(name string) bool {
	cmd := exec.Command("systemctl", "is-active", name)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) == "active"
}
