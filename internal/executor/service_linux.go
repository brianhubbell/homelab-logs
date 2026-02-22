//go:build linux

package executor

import (
	"fmt"
	"os/exec"
	"strings"
)

func isServiceRunning(name string) bool {
	cmd := exec.Command("systemctl", "is-active", name)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) == "active"
}

func serviceControl(service, op string) (map[string]interface{}, error) {
	var cmd *exec.Cmd

	switch op {
	case "status":
		cmd = exec.Command("systemctl", "is-active", service)
		out, err := cmd.Output()
		state := strings.TrimSpace(string(out))
		if state == "" {
			state = "unknown"
		}
		// is-active returns non-zero for inactive/failed, which is not an error for us
		_ = err
		return map[string]interface{}{
			"service": service,
			"state":   state,
		}, nil

	case "start":
		cmd = exec.Command("sudo", "systemctl", "start", service)
	case "stop":
		cmd = exec.Command("sudo", "systemctl", "stop", service)
	case "restart":
		cmd = exec.Command("sudo", "systemctl", "restart", service)
	default:
		return nil, fmt.Errorf("unknown service operation %q", op)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %s (%w)", op, service, strings.TrimSpace(string(out)), err)
	}

	// After start/stop/restart, get the current state
	stateCmd := exec.Command("systemctl", "is-active", service)
	stateOut, _ := stateCmd.Output()
	state := strings.TrimSpace(string(stateOut))
	if state == "" {
		state = "unknown"
	}

	return map[string]interface{}{
		"service": service,
		"state":   state,
	}, nil
}
