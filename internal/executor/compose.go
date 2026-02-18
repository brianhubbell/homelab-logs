package executor

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func composeControl(path, service, op string) (map[string]interface{}, error) {
	switch op {
	case "status":
		args := []string{"compose", "-f", path, "ps", "--format", "json"}
		cmd := exec.Command("docker", args...)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("compose status: %w", err)
		}
		// docker compose ps --format json outputs one JSON object per line
		var services []map[string]interface{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			var svc map[string]interface{}
			if err := json.Unmarshal([]byte(line), &svc); err != nil {
				continue
			}
			services = append(services, svc)
		}
		return map[string]interface{}{
			"path":     path,
			"services": services,
		}, nil

	case "start", "stop", "restart":
		args := []string{"compose", "-f", path, op}
		if service != "" {
			args = append(args, service)
		}
		cmd := exec.Command("docker", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("compose %s: %s (%w)", op, strings.TrimSpace(string(out)), err)
		}
		data := map[string]interface{}{
			"path":      path,
			"operation": op,
		}
		if service != "" {
			data["service"] = service
		}
		return data, nil

	default:
		return nil, fmt.Errorf("unknown compose operation %q", op)
	}
}
