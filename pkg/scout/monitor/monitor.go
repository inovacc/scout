// Package monitor provides continuous visual regression testing and site monitoring.
// It captures screenshots at intervals, compares against baselines, and reports changes.
package monitor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config configures a monitoring session.
type Config struct {
	URL      string        `json:"url"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Baseline string        `json:"baseline,omitempty"` // path to baseline screenshot
	OutputDir string       `json:"output_dir,omitempty"`
	Threshold float64      `json:"threshold,omitempty"` // 0.0-1.0, diff tolerance
}

// Result is the outcome of a single monitoring check.
type Result struct {
	URL        string    `json:"url"`
	Timestamp  time.Time `json:"timestamp"`
	Checksum   string    `json:"checksum"`     // SHA256 of screenshot
	DiffScore  float64   `json:"diff_score"`   // 0.0 = identical, 1.0 = completely different
	Changed    bool      `json:"changed"`
	Screenshot string    `json:"screenshot"`   // path to saved screenshot
	Error      string    `json:"error,omitempty"`
}

// ChangeHandler is called when a visual change is detected.
type ChangeHandler func(result Result)

// Baseline holds the reference screenshot and metadata.
type Baseline struct {
	URL       string    `json:"url"`
	Checksum  string    `json:"checksum"`
	CapturedAt time.Time `json:"captured_at"`
	Path      string    `json:"path"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
}

// BaselineManager manages reference screenshots for visual comparison.
type BaselineManager struct {
	dir string
	mu  sync.RWMutex
}

// NewBaselineManager creates a baseline manager using the given directory.
func NewBaselineManager(dir string) *BaselineManager {
	return &BaselineManager{dir: dir}
}

// baselineName returns a stable filename prefix for a URL.
func baselineName(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:8])
}

// Capture saves a screenshot as the baseline for a URL.
func (m *BaselineManager) Capture(url string, screenshot []byte) (*Baseline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, fmt.Errorf("monitor: create baseline dir: %w", err)
	}

	name := baselineName(url)
	path := filepath.Join(m.dir, name+".png")

	if err := os.WriteFile(path, screenshot, 0o600); err != nil {
		return nil, fmt.Errorf("monitor: save baseline: %w", err)
	}

	checksum := sha256.Sum256(screenshot)

	var width, height int

	if cfg, err := png.DecodeConfig(bytes.NewReader(screenshot)); err == nil {
		width = cfg.Width
		height = cfg.Height
	}

	b := &Baseline{
		URL:        url,
		Checksum:   hex.EncodeToString(checksum[:]),
		CapturedAt: time.Now().UTC(),
		Path:       path,
		Width:      width,
		Height:     height,
	}

	// Save metadata.
	meta, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("monitor: marshal baseline: %w", err)
	}

	_ = os.WriteFile(filepath.Join(m.dir, name+".json"), meta, 0o600)

	return b, nil
}

// Load reads the baseline for a URL.
func (m *BaselineManager) Load(url string) (*Baseline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name := baselineName(url)
	metaPath := filepath.Join(m.dir, name+".json")

	data, err := os.ReadFile(metaPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("monitor: no baseline for %s: %w", url, err)
	}

	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("monitor: parse baseline: %w", err)
	}

	return &b, nil
}

// List returns all saved baselines.
func (m *BaselineManager) List() ([]Baseline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var baselines []Baseline

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(m.dir, e.Name())) //nolint:gosec
		if err != nil {
			continue
		}

		var b Baseline
		if json.Unmarshal(data, &b) == nil {
			baselines = append(baselines, b)
		}
	}

	return baselines, nil
}

// Compare compares a screenshot against the baseline using pixel-level diff.
func Compare(baseline, current []byte, threshold float64) (float64, bool, error) {
	baseImg, err := png.Decode(bytes.NewReader(baseline))
	if err != nil {
		return 0, false, fmt.Errorf("monitor: decode baseline: %w", err)
	}

	currImg, err := png.Decode(bytes.NewReader(current))
	if err != nil {
		return 0, false, fmt.Errorf("monitor: decode current: %w", err)
	}

	score := diffScore(baseImg, currImg)
	changed := score > threshold

	return score, changed, nil
}

func diffScore(a, b image.Image) float64 {
	boundsA := a.Bounds()
	boundsB := b.Bounds()

	// Different sizes = definitely changed.
	if boundsA.Dx() != boundsB.Dx() || boundsA.Dy() != boundsB.Dy() {
		return 1.0
	}

	totalPixels := boundsA.Dx() * boundsA.Dy()
	if totalPixels == 0 {
		return 0
	}

	diffPixels := 0

	for y := boundsA.Min.Y; y < boundsA.Max.Y; y++ {
		for x := boundsA.Min.X; x < boundsA.Max.X; x++ {
			r1, g1, b1, _ := a.At(x, y).RGBA()
			r2, g2, b2, _ := b.At(x, y).RGBA()

			if r1 != r2 || g1 != g2 || b1 != b2 {
				diffPixels++
			}
		}
	}

	return float64(diffPixels) / float64(totalPixels)
}

// Monitor runs continuous monitoring checks.
type Monitor struct {
	config    Config
	baselines *BaselineManager
	logger    *slog.Logger
	onChange  ChangeHandler
	stopCh   chan struct{}
	stopOnce sync.Once
}

// New creates a new monitor.
func New(cfg Config, baselines *BaselineManager, onChange ChangeHandler) *Monitor {
	return &Monitor{
		config:    cfg,
		baselines: baselines,
		logger:    slog.New(slog.NewTextHandler(os.Stderr, nil)),
		onChange:  onChange,
		stopCh:   make(chan struct{}),
	}
}

// Stop stops the monitor. Safe to call multiple times.
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// Run starts the monitoring loop. Call Stop() to exit.
func (m *Monitor) Run(ctx context.Context, captureFunc func(url string) ([]byte, error)) error {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	m.logger.Info("monitor started", "url", m.config.URL, "interval", m.config.Interval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.stopCh:
			return nil
		case <-ticker.C:
			result := m.check(captureFunc)
			if result.Changed && m.onChange != nil {
				m.onChange(result)
			}
		}
	}
}

func (m *Monitor) check(captureFunc func(string) ([]byte, error)) Result {
	result := Result{
		URL:       m.config.URL,
		Timestamp: time.Now().UTC(),
	}

	screenshot, err := captureFunc(m.config.URL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	checksum := sha256.Sum256(screenshot)
	result.Checksum = hex.EncodeToString(checksum[:])

	// Save screenshot.
	if m.config.OutputDir != "" {
		_ = os.MkdirAll(m.config.OutputDir, 0o755)

		name := fmt.Sprintf("%s_%s.png", time.Now().Format("20060102_150405"), result.Checksum[:8])
		path := filepath.Join(m.config.OutputDir, name)

		if err := os.WriteFile(path, screenshot, 0o600); err == nil {
			result.Screenshot = path
		}
	}

	// Compare against baseline.
	baseline, err := m.baselines.Load(m.config.URL)
	if err != nil {
		// No baseline — auto-capture.
		_, _ = m.baselines.Capture(m.config.URL, screenshot)
		m.logger.Info("baseline captured", "url", m.config.URL)

		return result
	}

	// Short-circuit: identical checksums mean identical images.
	if result.Checksum == baseline.Checksum {
		return result
	}

	baselineData, err := os.ReadFile(baseline.Path) //nolint:gosec
	if err != nil {
		result.Error = fmt.Sprintf("read baseline: %s", err)
		return result
	}

	threshold := m.config.Threshold
	if threshold == 0 {
		threshold = 0.01 // 1% default
	}

	score, changed, err := Compare(baselineData, screenshot, threshold)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.DiffScore = score
	result.Changed = changed

	if changed {
		m.logger.Warn("visual change detected", "url", m.config.URL, "diff", fmt.Sprintf("%.2f%%", score*100))
	}

	return result
}
