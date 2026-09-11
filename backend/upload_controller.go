package backend

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type uploadControllerKey struct{}

// WithUploadController attaches an UploadController to the context.
func WithUploadController(ctx context.Context, c *UploadController) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, uploadControllerKey{}, c)
}

// UploadControllerFromContext extracts the UploadController from the context if present.
func UploadControllerFromContext(ctx context.Context) *UploadController {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(uploadControllerKey{}).(*UploadController); ok {
		return c
	}
	return nil
}

// UploadController coordinates bandwidth throttling and pause/resume across
// concurrent upload workers.
type UploadController struct {
	mu      sync.Mutex
	paused  bool
	pauseCh chan struct{} // open when paused, closed/nil when running

	limitBytesPerSec atomic.Int64 // 0 = unlimited

	tokenMu          sync.Mutex
	tokens           float64
	burst            float64
	lastTokenRefill  time.Time
}

// NewUploadController creates a new controller with an optional bandwidth limit in bytes/sec.
// limitBytesPerSec <= 0 means unlimited bandwidth.
func NewUploadController(limitBytesPerSec int64) *UploadController {
	c := &UploadController{
		lastTokenRefill: time.Now(),
	}
	c.SetLimit(limitBytesPerSec)
	return c
}

// Pause pauses uploads. In-flight readers block until Resume is called.
func (c *UploadController) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		c.paused = true
		c.pauseCh = make(chan struct{})
	}
}

// Resume resumes paused uploads.
func (c *UploadController) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		c.paused = false
		if c.pauseCh != nil {
			close(c.pauseCh)
			c.pauseCh = nil
		}
	}
}

// IsPaused returns whether the upload is currently paused.
func (c *UploadController) IsPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// WaitIfPaused blocks if uploads are paused, until Resume is called or ctx is cancelled.
func (c *UploadController) WaitIfPaused(ctx context.Context) error {
	for {
		c.mu.Lock()
		ch := c.pauseCh
		c.mu.Unlock()

		if ch == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
			// Unpaused, continue loop to verify state
		}
	}
}

// SetLimit updates the maximum upload bandwidth in bytes per second.
// Pass <= 0 for unlimited.
func (c *UploadController) SetLimit(bytesPerSec int64) {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	c.limitBytesPerSec.Store(bytesPerSec)

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if bytesPerSec > 0 {
		burst := float64(bytesPerSec)
		if burst < 256*1024 {
			burst = 256 * 1024
		}
		c.burst = burst
		if c.tokens > burst {
			c.tokens = burst
		}
	} else {
		c.tokens = 0
		c.burst = 0
	}
	c.lastTokenRefill = time.Now()
}

// GetLimit returns the configured bandwidth limit in bytes per second (0 = unlimited).
func (c *UploadController) GetLimit() int64 {
	return c.limitBytesPerSec.Load()
}

// Throttle consumes tokens for the specified byte count or sleeps until available.
// If paused, it waits until resumed.
func (c *UploadController) Throttle(ctx context.Context, bytesCount int) error {
	if bytesCount <= 0 {
		return nil
	}

	for {
		if err := c.WaitIfPaused(ctx); err != nil {
			return err
		}

		limit := c.limitBytesPerSec.Load()
		if limit <= 0 {
			return nil
		}

		c.tokenMu.Lock()
		now := time.Now()
		elapsed := now.Sub(c.lastTokenRefill).Seconds()
		c.lastTokenRefill = now

		c.tokens += elapsed * float64(limit)
		if c.tokens > c.burst {
			c.tokens = c.burst
		}

		if c.tokens >= float64(bytesCount) {
			c.tokens -= float64(bytesCount)
			c.tokenMu.Unlock()
			return nil
		}

		needed := float64(bytesCount) - c.tokens
		sleepSec := needed / float64(limit)
		// Cap sleep interval to 50ms so limit changes, pause, and cancellation are responsive
		if sleepSec > 0.05 {
			sleepSec = 0.05
		}
		c.tokenMu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(sleepSec * float64(time.Second))):
		}
	}
}

// ThrottledReader wraps an io.Reader, applying pause/resume and rate limiting
// from an UploadController, as well as tracking progress.
type ThrottledReader struct {
	ctx          context.Context
	reader       io.Reader
	controller   *UploadController
	total        int64
	read         atomic.Int64
	lastEmit     time.Time
	emitInterval time.Duration
	onProgress   func(bytesRead, totalBytes int64)
}

// NewThrottledReader creates a new ThrottledReader.
func NewThrottledReader(
	ctx context.Context,
	reader io.Reader,
	total int64,
	controller *UploadController,
	onProgress func(bytesRead, totalBytes int64),
) *ThrottledReader {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ThrottledReader{
		ctx:          ctx,
		reader:       reader,
		controller:   controller,
		total:        total,
		emitInterval: 100 * time.Millisecond,
		onProgress:   onProgress,
	}
}

// Read reads from the underlying reader while honoring pause, bandwidth limit,
// and progress reporting.
func (tr *ThrottledReader) Read(p []byte) (n int, err error) {
	if tr.controller != nil {
		if waitErr := tr.controller.WaitIfPaused(tr.ctx); waitErr != nil {
			return 0, waitErr
		}
	}

	// Limit buffer size per read when throttled to ensure fine-grained rate control
	readBuf := p
	if tr.controller != nil && tr.controller.GetLimit() > 0 {
		maxChunk := 32 * 1024
		if len(readBuf) > maxChunk {
			readBuf = readBuf[:maxChunk]
		}
	}

	n, err = tr.reader.Read(readBuf)
	if n > 0 {
		currentRead := tr.read.Add(int64(n))

		if tr.controller != nil {
			if throttleErr := tr.controller.Throttle(tr.ctx, n); throttleErr != nil {
				return n, throttleErr
			}
		}

		now := time.Now()
		if now.Sub(tr.lastEmit) >= tr.emitInterval || err == io.EOF {
			tr.lastEmit = now
			if tr.onProgress != nil {
				tr.onProgress(currentRead, tr.total)
			}
		}
	}

	return n, err
}

// BytesRead returns total bytes read through this reader so far.
func (tr *ThrottledReader) BytesRead() int64 {
	return tr.read.Load()
}
