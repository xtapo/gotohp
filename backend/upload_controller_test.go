package backend

import (
	"bytes"
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func TestUploadControllerPauseResume(t *testing.T) {
	c := NewUploadController(0)
	if c.IsPaused() {
		t.Fatal("expected controller to start unpaused")
	}

	c.Pause()
	if !c.IsPaused() {
		t.Fatal("expected controller to be paused")
	}

	// Verify WaitIfPaused blocks while paused
	unblocked := make(chan struct{})
	go func() {
		err := c.WaitIfPaused(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		close(unblocked)
	}()

	select {
	case <-unblocked:
		t.Fatal("WaitIfPaused should have blocked while paused")
	case <-time.After(50 * time.Millisecond):
		// Expected to block
	}

	// Resume should unblock WaitIfPaused
	c.Resume()
	if c.IsPaused() {
		t.Fatal("expected controller to be unpaused")
	}

	select {
	case <-unblocked:
		// Succeeded
	case <-time.After(200 * time.Millisecond):
		t.Fatal("WaitIfPaused did not unblock after Resume")
	}
}

func TestUploadControllerContextCancellationWhilePaused(t *testing.T) {
	c := NewUploadController(0)
	c.Pause()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- c.WaitIfPaused(ctx)
	}()

	select {
	case <-errCh:
		t.Fatal("WaitIfPaused should block")
	case <-time.After(50 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("WaitIfPaused did not unblock on context cancellation")
	}
}

func TestUploadControllerThrottling(t *testing.T) {
	// Limit to 100 KB/s
	limit := int64(100 * 1024)
	c := NewUploadController(limit)
	// Drain any initial burst
	c.tokens = 0

	data := bytes.Repeat([]byte("A"), 50*1024) // 50 KB -> should take ~0.5s
	start := time.Now()

	err := c.Throttle(context.Background(), len(data))
	if err != nil {
		t.Fatalf("Throttle error: %v", err)
	}

	elapsed := time.Since(start)
	// 50KB / 100KB/s = ~0.5s. Expect at least 350ms
	if elapsed < 350*time.Millisecond {
		t.Errorf("Throttle elapsed %v, expected at least 350ms", elapsed)
	}
}

func TestThrottledReaderPauseAndProgress(t *testing.T) {
	controller := NewUploadController(0)
	data := []byte("Hello, World! This is a test for ThrottledReader.")
	var progressCalls atomic.Int32

	onProgress := func(read, total int64) {
		progressCalls.Add(1)
	}

	reader := NewThrottledReader(context.Background(), bytes.NewReader(data), int64(len(data)), controller, onProgress)

	buf := make([]byte, 10)
	n, err := reader.Read(buf)
	if err != nil || n != 10 {
		t.Fatalf("read %d bytes, err %v", n, err)
	}
	if string(buf[:n]) != "Hello, Wor" {
		t.Fatalf("unexpected content: %s", string(buf[:n]))
	}

	// Now pause
	controller.Pause()
	readDone := make(chan struct{})

	go func() {
		buf2 := make([]byte, 10)
		_, _ = reader.Read(buf2)
		close(readDone)
	}()

	select {
	case <-readDone:
		t.Fatal("read should be blocked while controller is paused")
	case <-time.After(50 * time.Millisecond):
		// Blocked as expected
	}

	// Resume
	controller.Resume()

	select {
	case <-readDone:
		// Successfully unblocked and read
	case <-time.After(200 * time.Millisecond):
		t.Fatal("read did not resume after controller.Resume()")
	}

	// Read remaining to EOF
	allRemaining, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if len(allRemaining)+20 != len(data) {
		t.Fatalf("total read length mismatch")
	}
}
