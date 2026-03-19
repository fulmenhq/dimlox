package transfer

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type progressReporter struct {
	label       string
	out         io.Writer
	total       int64
	interactive bool
	start       time.Time
	current     atomic.Int64
	done        chan struct{}
	once        sync.Once
}

func newProgressReporter(label string, total int64) *progressReporter {
	stderr := os.Stderr
	return &progressReporter{
		label:       label,
		out:         stderr,
		total:       total,
		interactive: isTerminal(os.Stdout),
		start:       time.Now(),
		done:        make(chan struct{}),
	}
}

func (r *progressReporter) Observe(total int64) {
	if total < 0 {
		return
	}
	for {
		current := r.current.Load()
		if total <= current {
			return
		}
		if r.current.CompareAndSwap(current, total) {
			return
		}
	}
}

func (r *progressReporter) Start() {
	if r == nil || r.out == nil {
		return
	}
	go r.loop()
}

func (r *progressReporter) Finish() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		close(r.done)
		if r.out != nil {
			if r.interactive {
				_, _ = fmt.Fprintln(r.out)
			} else {
				r.printStructured(true)
			}
		}
	})
}

func (r *progressReporter) loop() {
	interval := 500 * time.Millisecond
	if !r.interactive {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if r.interactive {
				r.printInteractive()
			} else {
				r.printStructured(false)
			}
		case <-r.done:
			return
		}
	}
}

func (r *progressReporter) printInteractive() {
	current := r.current.Load()
	elapsed := time.Since(r.start)
	speed := mbPerSecond(current, elapsed)
	pct, eta := pctAndETA(current, r.total, elapsed)
	if r.total > 0 {
		_, _ = fmt.Fprintf(r.out, "\r%s %.1f%% %s/%s (%.1f MB/s) ETA %s", r.label, pct, humanBytes(current), humanBytes(r.total), speed, eta.Round(time.Second))
		return
	}
	_, _ = fmt.Fprintf(r.out, "\r%s %s transferred (%.1f MB/s)", r.label, humanBytes(current), speed)
}

func (r *progressReporter) printStructured(final bool) {
	current := r.current.Load()
	elapsed := time.Since(r.start)
	speed := mbPerSecond(current, elapsed)
	pct, eta := pctAndETA(current, r.total, elapsed)
	status := "progress"
	if final {
		status = "done"
	}
	if r.total > 0 {
		_, _ = fmt.Fprintf(r.out, "%s label=%s bytes=%d total=%d pct=%.1f mbps=%.1f eta=%s\n", status, r.label, current, r.total, pct, speed, eta.Round(time.Second))
		return
	}
	_, _ = fmt.Fprintf(r.out, "%s label=%s bytes=%d mbps=%.1f\n", status, r.label, current, speed)
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func pctAndETA(current, total int64, elapsed time.Duration) (float64, time.Duration) {
	if total <= 0 || current <= 0 {
		return 0, 0
	}
	pct := float64(current) * 100 / float64(total)
	rate := float64(current) / elapsed.Seconds()
	if rate <= 0 {
		return pct, 0
	}
	remaining := float64(total-current) / rate
	if remaining < 0 {
		remaining = 0
	}
	return pct, time.Duration(remaining * float64(time.Second))
}

func mbPerSecond(current int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(current) / 1024 / 1024 / elapsed.Seconds()
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
