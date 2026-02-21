package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const autoUpdateRepo = "https://github.com/brianhubbell/homelab-agent.git"

// SelfUpdate checks for a new version of homelab-agent and updates if needed.
func (e *Executor) SelfUpdate() (map[string]interface{}, error) {
	results := map[string]interface{}{
		"service": "homelab-agent",
	}
	steps := []map[string]interface{}{}

	repoDir := filepath.Join(e.DeployDir, "homelab-agent")

	// 1. Ensure repo exists
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		cmd := exec.Command("git", "fetch", "origin")
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			steps = append(steps, map[string]interface{}{
				"step": "git_fetch", "status": "error", "detail": strings.TrimSpace(string(out)),
			})
			results["steps"] = steps
			return nil, fmt.Errorf("git fetch failed: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		steps = append(steps, map[string]interface{}{
			"step": "git_fetch", "status": "ok",
		})
	} else {
		cmd := exec.Command("git", "clone", autoUpdateRepo, repoDir)
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

	// 2. Check version
	cmd := exec.Command("git", "describe", "--always", "origin/main")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		steps = append(steps, map[string]interface{}{
			"step": "check_version", "status": "error", "detail": strings.TrimSpace(string(out)),
		})
		results["steps"] = steps
		return nil, fmt.Errorf("version check failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	newVersion := strings.TrimSpace(string(out))
	steps = append(steps, map[string]interface{}{
		"step": "check_version", "status": "ok", "detail": newVersion,
	})

	if newVersion == e.CurrentVersion {
		steps = append(steps, map[string]interface{}{
			"step": "compare", "status": "up-to-date", "detail": newVersion,
		})
		results["steps"] = steps
		results["status"] = "up-to-date"
		results["version"] = newVersion
		return results, nil
	}

	// 3. Pull latest
	cmd = exec.Command("git", "pull")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
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

	// 4. Build
	ldflags := fmt.Sprintf("-X main.Version=%s", newVersion)
	cmd = exec.Command("go", "build", "-ldflags", ldflags, "-o", "build/bin/homelab-agent", "./cmd/homelab-agent/")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err = cmd.CombinedOutput()
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

	// 5. Install binary
	binarySource := filepath.Join(repoDir, "build/bin/homelab-agent")
	cpCmd := exec.Command("sudo", "cp", binarySource, "/usr/local/bin/homelab-agent")
	out, err = cpCmd.CombinedOutput()
	if err != nil {
		steps = append(steps, map[string]interface{}{
			"step": "install_binary", "status": "error", "detail": strings.TrimSpace(string(out)),
		})
		results["steps"] = steps
		return nil, fmt.Errorf("install binary failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	steps = append(steps, map[string]interface{}{
		"step": "install_binary", "status": "ok",
	})

	// 6. Restart service (service manager kills current process, starts new binary)
	if err := restartService("homelab-agent"); err != nil {
		steps = append(steps, map[string]interface{}{
			"step": "restart", "status": "error", "detail": err.Error(),
		})
		results["steps"] = steps
		return nil, fmt.Errorf("restart failed: %w", err)
	}
	steps = append(steps, map[string]interface{}{
		"step": "restart", "status": "ok",
	})

	results["steps"] = steps
	results["status"] = "updated"
	results["old_version"] = e.CurrentVersion
	results["new_version"] = newVersion
	return results, nil
}
