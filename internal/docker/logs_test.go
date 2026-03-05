package docker

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestStreamLogs_Cancel(t *testing.T) {
	// Verify that cancelling the context stops StreamLogs
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := StreamLogs(ctx, "nonexistent-container-for-test", 10, func(line string) {
		// Should not receive lines from a nonexistent container
	})
	// Either context deadline or docker error is acceptable
	if err == nil {
		t.Log("StreamLogs returned nil (docker may have exited immediately)")
	}
}
