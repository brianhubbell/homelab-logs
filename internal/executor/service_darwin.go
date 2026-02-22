//go:build darwin

package executor

import (
	"fmt"
	"os/exec"
	"strings"
)

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

func serviceControl(service, op string) (map[string]interface{}, error) {
	label := "com." + service

	switch op {
	case "status":
		cmd := exec.Command("launchctl", "list")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("launchctl list: %w", err)
		}
		state := "inactive"
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, label) {
				state = "active"
				break
			}
		}
		return map[string]interface{}{
			"service": service,
			"state":   state,
		}, nil

	case "start":
		cmd := exec.Command("launchctl", "start", label)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("start %s: %s (%w)", service, strings.TrimSpace(string(out)), err)
		}

	case "stop":
		cmd := exec.Command("launchctl", "stop", label)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("stop %s: %s (%w)", service, strings.TrimSpace(string(out)), err)
		}

	case "restart":
		stopCmd := exec.Command("launchctl", "stop", label)
		stopCmd.CombinedOutput()
		startCmd := exec.Command("launchctl", "start", label)
		out, err := startCmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("restart %s: %s (%w)", service, strings.TrimSpace(string(out)), err)
		}

	default:
		return nil, fmt.Errorf("unknown service operation %q", op)
	}

	// Check state after operation
	listCmd := exec.Command("launchctl", "list")
	listOut, _ := listCmd.Output()
	state := "inactive"
	for _, line := range strings.Split(string(listOut), "\n") {
		if strings.Contains(line, label) {
			state = "active"
			break
		}
	}

	return map[string]interface{}{
		"service": service,
		"state":   state,
	}, nil
}
