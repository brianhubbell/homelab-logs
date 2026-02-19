package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// serviceCheck checks if a service binary is installed and whether the service is running.
func (e *Executor) serviceCheck(service string) (map[string]interface{}, error) {
	installPath := filepath.Join("/usr/local/bin", service)

	installed := false
	if _, err := os.Stat(installPath); err == nil {
		installed = true
	}

	running := isServiceRunning(service)

	return map[string]interface{}{
		"service":    service,
		"installed":  installed,
		"running":    running,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
	}, nil
}

// serviceDeploy runs the full deploy pipeline for a service.
func (e *Executor) serviceDeploy(args map[string]string) (map[string]interface{}, error) {
	service := args["service"]
	repo := args["repo"]
	buildCmd := args["build_cmd"]
	binaryName := args["binary_name"]
	installPath := args["install_path"]
	systemdUnit := args["systemd_unit"]
	launchdUnit := args["launchd_unit"]
	postInstall := args["post_install"]

	if service == "" || repo == "" || buildCmd == "" || binaryName == "" || installPath == "" {
		return nil, fmt.Errorf("missing required deploy args: service, repo, build_cmd, binary_name, install_path")
	}

	results := map[string]interface{}{
		"service": service,
	}
	steps := []map[string]interface{}{}

	// 1. Check if service is running, stop if so
	if isServiceRunning(service) {
		if err := stopService(service); err != nil {
			steps = append(steps, map[string]interface{}{
				"step": "stop_service", "status": "error", "detail": err.Error(),
			})
			results["steps"] = steps
			return nil, fmt.Errorf("failed to stop running service: %w", err)
		}
		steps = append(steps, map[string]interface{}{
			"step": "stop_service", "status": "ok",
		})
	} else {
		steps = append(steps, map[string]interface{}{
			"step": "stop_service", "status": "skipped", "detail": "service not running",
		})
	}

	// 2. Clone or pull repo
	repoDir := filepath.Join(e.DeployDir, service)
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		// Repo exists, pull
		cmd := exec.Command("git", "pull")
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			steps = append(steps, map[string]interface{}{
				"step": "git_pull", "status": "error", "detail": strings.TrimSpace(string(out)),
			})
			results["steps"] = steps
			return nil, fmt.Errorf("git pull failed: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		steps = append(steps, map[string]interface{}{
			"step": "git_pull", "status": "ok", "detail": strings.TrimSpace(string(out)),
		})
	} else {
		// Clone
		cmd := exec.Command("git", "clone", repo, repoDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			steps = append(steps, map[string]interface{}{
				"step": "git_clone", "status": "error", "detail": strings.TrimSpace(string(out)),
			})
			results["steps"] = steps
			return nil, fmt.Errorf("git clone failed: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		steps = append(steps, map[string]interface{}{
			"step": "git_clone", "status": "ok",
		})
	}

	// 3. Run build command
	buildParts := strings.Fields(buildCmd)
	cmd := exec.Command(buildParts[0], buildParts[1:]...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		steps = append(steps, map[string]interface{}{
			"step": "build", "status": "error", "detail": strings.TrimSpace(string(out)),
		})
		results["steps"] = steps
		return nil, fmt.Errorf("build failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	steps = append(steps, map[string]interface{}{
		"step": "build", "status": "ok",
	})

	// 4. Copy binary to install path
	binarySource := filepath.Join(repoDir, binaryName)
	cpCmd := exec.Command("sudo", "cp", binarySource, installPath)
	out, err = cpCmd.CombinedOutput()
	if err != nil {
		steps = append(steps, map[string]interface{}{
			"step": "install_binary", "status": "error", "detail": strings.TrimSpace(string(out)),
		})
		results["steps"] = steps
		return nil, fmt.Errorf("copy binary failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	steps = append(steps, map[string]interface{}{
		"step": "install_binary", "status": "ok", "detail": installPath,
	})

	// 5. Install service unit file (platform-specific)
	unitFile := systemdUnit
	if runtime.GOOS == "darwin" {
		unitFile = launchdUnit
	}
	if unitFile != "" {
		if err := installServiceUnit(service, unitFile, repoDir); err != nil {
			steps = append(steps, map[string]interface{}{
				"step": "install_unit", "status": "error", "detail": err.Error(),
			})
			results["steps"] = steps
			return nil, fmt.Errorf("install service unit failed: %w", err)
		}
		steps = append(steps, map[string]interface{}{
			"step": "install_unit", "status": "ok",
		})
	} else {
		steps = append(steps, map[string]interface{}{
			"step": "install_unit", "status": "skipped", "detail": "no unit file specified",
		})
	}

	// 6. Run post-install command if provided
	if postInstall != "" {
		postParts := strings.Fields(postInstall)
		postCmd := exec.Command(postParts[0], postParts[1:]...)
		postCmd.Dir = repoDir
		out, err := postCmd.CombinedOutput()
		if err != nil {
			steps = append(steps, map[string]interface{}{
				"step": "post_install", "status": "error", "detail": strings.TrimSpace(string(out)),
			})
			results["steps"] = steps
			return nil, fmt.Errorf("post-install failed: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		steps = append(steps, map[string]interface{}{
			"step": "post_install", "status": "ok",
		})
	} else {
		steps = append(steps, map[string]interface{}{
			"step": "post_install", "status": "skipped",
		})
	}

	// 7. Enable and start service
	if err := enableAndStartService(service); err != nil {
		steps = append(steps, map[string]interface{}{
			"step": "enable_start", "status": "error", "detail": err.Error(),
		})
		results["steps"] = steps
		return nil, fmt.Errorf("enable and start service failed: %w", err)
	}
	steps = append(steps, map[string]interface{}{
		"step": "enable_start", "status": "ok",
	})

	// 8. Add service to whitelist
	e.allowedServices[service] = true
	steps = append(steps, map[string]interface{}{
		"step": "whitelist", "status": "ok",
	})

	results["steps"] = steps
	return results, nil
}
