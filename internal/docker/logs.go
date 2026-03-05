package docker

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
)

// StreamLogs runs `docker logs -f --tail <tail> <container>` and calls handler for each line.
// Returns when context is cancelled or the process exits.
func StreamLogs(ctx context.Context, container string, tail int, handler func(line string)) error {
	tailStr := fmt.Sprintf("%d", tail)
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", tailStr, container)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	// Merge stderr into stdout so we capture all output
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		handler(scanner.Text())
	}

	return cmd.Wait()
}
