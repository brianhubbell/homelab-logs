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

// isDockerContainer returns true if a container with the given name exists.
func isDockerContainer(name string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{.Id}}", name)
	return cmd.Run() == nil
}

// dockerActiveState maps a Docker container status to an active_state string.
func dockerActiveState(name string) string {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", name)
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	switch strings.TrimSpace(string(out)) {
	case "running":
		return "active"
	case "restarting":
		return "activating"
	default:
		return "inactive"
	}
}

func serviceControl(service, op string) (map[string]interface{}, error) {
	if isDockerContainer(service) {
		return dockerServiceControl(service, op)
	}
	return systemdServiceControl(service, op)
}

func dockerServiceControl(service, op string) (map[string]interface{}, error) {
	switch op {
	case "status":
		return map[string]interface{}{
			"service":      service,
			"active_state": dockerActiveState(service),
		}, nil
	case "start", "stop", "restart":
		cmd := exec.Command("docker", op, service)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("docker %s %s: %s (%w)", op, service, strings.TrimSpace(string(out)), err)
		}
		return map[string]interface{}{
			"service":      service,
			"active_state": dockerActiveState(service),
		}, nil
	default:
		return nil, fmt.Errorf("unknown service operation %q", op)
	}
}

func systemdServiceControl(service, op string) (map[string]interface{}, error) {
	switch op {
	case "status":
		cmd := exec.Command("systemctl", "is-active", service)
		out, _ := cmd.Output()
		state := strings.TrimSpace(string(out))
		if state == "" {
			state = "unknown"
		}
		return map[string]interface{}{
			"service":      service,
			"active_state": state,
		}, nil
	case "start", "stop", "restart":
		cmd := exec.Command("sudo", "systemctl", op, service)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("%s %s: %s (%w)", op, service, strings.TrimSpace(string(out)), err)
		}
		stateCmd := exec.Command("systemctl", "is-active", service)
		stateOut, _ := stateCmd.Output()
		state := strings.TrimSpace(string(stateOut))
		if state == "" {
			state = "unknown"
		}
		return map[string]interface{}{
			"service":      service,
			"active_state": state,
		}, nil
	default:
		return nil, fmt.Errorf("unknown service operation %q", op)
	}
}
