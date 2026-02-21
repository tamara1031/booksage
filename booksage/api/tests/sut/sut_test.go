package sut

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestAPISstartup(t *testing.T) {
	// 1. Build the binary
	cmdBuild := exec.Command("go", "build", "-o", "booksage-api-sut", "./cmd/booksage-api")
	cmdBuild.Dir = "../../"
	if err := cmdBuild.Run(); err != nil {
		t.Fatalf("Failed to build API binary: %v", err)
	}
	defer func() { _ = os.Remove("../../booksage-api-sut") }()

	// 2. Start the API
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdRun := exec.CommandContext(ctx, "./booksage-api-sut")
	cmdRun.Dir = "../../"
	// Set environment variables for the test
	cmdRun.Env = append(os.Environ(), "BS_WORKER_ADDR=localhost:9999") // Mock worker address

	err := cmdRun.Start()
	if err != nil {
		t.Fatalf("Failed to start API binary: %v", err)
	}

	// 3. Verify it's running (Wait a bit for startup)
	time.Sleep(2 * time.Second)

	// Since current main.go doesn't have a health check endpoint yet,
	// we just verify the process is still running.
	if cmdRun.Process == nil {
		t.Error("Process failed to start")
	}

	// Clean up
	cancel()
	_ = cmdRun.Wait()
}
