package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ServiceVersion runs the --version flag on a service binary and returns the version string.
func (e *Executor) ServiceVersion(service string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join("/usr/local/bin", service), "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ServiceVersions returns a map of service name → version for configured services.
func (e *Executor) ServiceVersions() map[string]string {
	versions := make(map[string]string)
	for _, svc := range e.Services {
		if v := e.ServiceVersion(svc); v != "" {
			versions[svc] = v
		}
	}
	return versions
}

// serviceCheck checks if a service binary is installed and whether the service is running.
func (e *Executor) serviceCheck(service string) (map[string]interface{}, error) {
	installPath := filepath.Join("/usr/local/bin", service)

	installed := false
	if _, err := os.Stat(installPath); err == nil {
		installed = true
	}

	running := isServiceRunning(service)

	result := map[string]interface{}{
		"service":    service,
		"installed":  installed,
		"running":    running,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
	}
	if installed {
		result["version"] = e.ServiceVersion(service)
	}
	return result, nil
}

