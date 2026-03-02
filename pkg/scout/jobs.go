package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AsyncJobStatus represents the state of an async job.
type AsyncJobStatus string

const (
	AsyncJobPending   AsyncJobStatus = "pending"
	AsyncJobRunning   AsyncJobStatus = "running"
	AsyncJobCompleted AsyncJobStatus = "completed"
	AsyncJobFailed    AsyncJobStatus = "failed"
	AsyncJobCancelled AsyncJobStatus = "cancelled"
)

// AsyncJob represents a long-running operation such as batch scraping or crawling.
type AsyncJob struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Status    AsyncJobStatus   `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	StartedAt *time.Time       `json:"started_at,omitempty"`
	EndedAt   *time.Time       `json:"ended_at,omitempty"`
	Progress  AsyncJobProgress `json:"progress"`
	Error     string           `json:"error,omitempty"`
	Result    any              `json:"result,omitempty"`
	Config    any              `json:"config,omitempty"`
}

// AsyncJobProgress tracks completion of a job's units of work.
type AsyncJobProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// AsyncJobManager manages async jobs with persistent state on disk.
type AsyncJobManager struct {
	dir    string
	mu     sync.RWMutex
	jobs   map[string]*AsyncJob
	cancel map[string]context.CancelFunc
}

// NewAsyncJobManager creates a job manager that persists jobs to dir.
// It creates the directory if it does not exist and loads any existing jobs.
func NewAsyncJobManager(dir string) (*AsyncJobManager, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("scout: jobs: create dir: %w", err)
	}

	m := &AsyncJobManager{
		dir:    dir,
		jobs:   make(map[string]*AsyncJob),
		cancel: make(map[string]context.CancelFunc),
	}

	if err := m.load(); err != nil {
		return nil, fmt.Errorf("scout: jobs: load: %w", err)
	}

	return m, nil
}

// Create creates a new pending job and returns its ID.
func (m *AsyncJobManager) Create(jobType string, config any) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	j := &AsyncJob{
		ID:        uuid.New().String(),
		Type:      jobType,
		Status:    AsyncJobPending,
		CreatedAt: time.Now().UTC(),
		Config:    config,
	}

	m.jobs[j.ID] = j

	if err := m.save(j); err != nil {
		delete(m.jobs, j.ID)
		return "", err
	}

	return j.ID, nil
}

// Start marks a job as running.
func (m *AsyncJobManager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("scout: jobs: not found: %s", id)
	}

	if j.Status != AsyncJobPending {
		return fmt.Errorf("scout: jobs: cannot start job in %s state", j.Status)
	}

	now := time.Now().UTC()
	j.Status = AsyncJobRunning
	j.StartedAt = &now

	return m.save(j)
}

// UpdateProgress updates the progress counters of a running job.
func (m *AsyncJobManager) UpdateProgress(id string, completed, failed int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("scout: jobs: not found: %s", id)
	}

	j.Progress.Completed = completed
	j.Progress.Failed = failed

	return m.save(j)
}

// Complete marks a job as completed with a result.
func (m *AsyncJobManager) Complete(id string, result any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("scout: jobs: not found: %s", id)
	}

	now := time.Now().UTC()
	j.Status = AsyncJobCompleted
	j.EndedAt = &now
	j.Result = result

	return m.save(j)
}

// Fail marks a job as failed with an error message.
func (m *AsyncJobManager) Fail(id string, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("scout: jobs: not found: %s", id)
	}

	now := time.Now().UTC()
	j.Status = AsyncJobFailed
	j.EndedAt = &now
	j.Error = errMsg

	return m.save(j)
}

// Cancel cancels a running job. If a cancel function was registered, it is called.
func (m *AsyncJobManager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("scout: jobs: not found: %s", id)
	}

	if j.Status != AsyncJobRunning && j.Status != AsyncJobPending {
		return fmt.Errorf("scout: jobs: cannot cancel job in %s state", j.Status)
	}

	if fn, ok := m.cancel[id]; ok {
		fn()
		delete(m.cancel, id)
	}

	now := time.Now().UTC()
	j.Status = AsyncJobCancelled
	j.EndedAt = &now

	return m.save(j)
}

// Get returns a job by ID.
func (m *AsyncJobManager) Get(id string) (*AsyncJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	j, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("scout: jobs: not found: %s", id)
	}

	// Return a copy to avoid data races.
	cp := *j

	return &cp, nil
}

// List returns all jobs, optionally filtered by status. Jobs are sorted by creation time descending.
func (m *AsyncJobManager) List(status ...AsyncJobStatus) []*AsyncJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filter := make(map[AsyncJobStatus]bool, len(status))
	for _, s := range status {
		filter[s] = true
	}

	var result []*AsyncJob

	for _, j := range m.jobs {
		if len(filter) > 0 && !filter[j.Status] {
			continue
		}

		cp := *j
		result = append(result, &cp)
	}

	sort.Slice(result, func(i, k int) bool {
		return result[i].CreatedAt.After(result[k].CreatedAt)
	})

	return result
}

// RegisterCancel registers a context cancel function for a running job.
func (m *AsyncJobManager) RegisterCancel(id string, fn context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cancel[id] = fn
}

func (m *AsyncJobManager) save(j *AsyncJob) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: jobs: marshal: %w", err)
	}

	path := filepath.Join(m.dir, j.ID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: jobs: write: %w", err)
	}

	return nil
}

func (m *AsyncJobManager) load() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("scout: jobs: read dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}

		var j AsyncJob
		if err := json.Unmarshal(data, &j); err != nil {
			continue
		}

		m.jobs[j.ID] = &j
	}

	return nil
}
