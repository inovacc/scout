package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the state of an LLM extraction job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusExtracting JobStatus = "extracting"
	JobStatusReviewing  JobStatus = "reviewing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// LLMSession tracks a named workspace for LLM jobs.
type LLMSession struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SessionIndex is the top-level sessions.json file.
type SessionIndex struct {
	Sessions []LLMSession `json:"sessions"`
	Current  string       `json:"current"`
}

// LLMJob tracks a single extraction+review job with full metadata.
type LLMJob struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Status    JobStatus `json:"status"`

	// Input
	URL    string `json:"url"`
	Prompt string `json:"prompt"`

	// Extract phase
	ExtractProvider string `json:"extract_provider"`
	ExtractModel    string `json:"extract_model"`
	ExtractResult   string `json:"extract_result,omitempty"`

	// Review phase
	ReviewProvider string `json:"review_provider,omitempty"`
	ReviewModel    string `json:"review_model,omitempty"`
	ReviewPrompt   string `json:"review_prompt,omitempty"`
	ReviewResult   string `json:"review_result,omitempty"`

	// Timing
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ExtractStarted  time.Time `json:"extract_started,omitempty"`
	ExtractFinished time.Time `json:"extract_finished,omitempty"`
	ReviewStarted   time.Time `json:"review_started,omitempty"`
	ReviewFinished  time.Time `json:"review_finished,omitempty"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// JobRef is a lightweight reference stored in the jobs index.
type JobRef struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Status    JobStatus `json:"status"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// JobIndex is jobs/jobs.json — the index of all jobs and the current active job.
type JobIndex struct {
	Jobs    []JobRef `json:"jobs"`
	Current string   `json:"current"`
}

// LLMWorkspace manages session and job state on the filesystem.
//
// Directory structure:
//
//	<root>/
//	  sessions.json
//	  jobs/
//	    jobs.json
//	    <uuid>/
//	      job.json
//	      extract.md
//	      review.md
type LLMWorkspace struct {
	root string
}

// NewLLMWorkspace creates or opens a workspace at the given path.
func NewLLMWorkspace(path string) (*LLMWorkspace, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("scout: workspace: resolve path: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(abs, "jobs"), 0o755); err != nil {
		return nil, fmt.Errorf("scout: workspace: create dirs: %w", err)
	}

	ws := &LLMWorkspace{root: abs}

	// Initialize sessions.json if missing
	sessPath := filepath.Join(abs, "sessions.json")
	if _, err := os.Stat(sessPath); os.IsNotExist(err) {
		if err := ws.writeJSON(sessPath, &SessionIndex{}); err != nil {
			return nil, err
		}
	}

	// Initialize jobs/jobs.json if missing
	jobsPath := filepath.Join(abs, "jobs", "jobs.json")
	if _, err := os.Stat(jobsPath); os.IsNotExist(err) {
		if err := ws.writeJSON(jobsPath, &JobIndex{}); err != nil {
			return nil, err
		}
	}

	return ws, nil
}

// Root returns the workspace root path.
func (w *LLMWorkspace) Root() string { return w.root }

// CreateSession creates a new named session and sets it as current.
func (w *LLMWorkspace) CreateSession(name string, meta map[string]string) (*LLMSession, error) {
	idx, err := w.readSessionIndex()
	if err != nil {
		return nil, err
	}

	sess := LLMSession{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metadata:  meta,
	}

	idx.Sessions = append(idx.Sessions, sess)
	idx.Current = sess.ID

	if err := w.writeJSON(w.sessionsPath(), idx); err != nil {
		return nil, err
	}

	return &sess, nil
}

// GetSession returns a session by ID.
func (w *LLMWorkspace) GetSession(id string) (*LLMSession, error) {
	idx, err := w.readSessionIndex()
	if err != nil {
		return nil, err
	}

	for _, s := range idx.Sessions {
		if s.ID == id {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("scout: workspace: session %q not found", id)
}

// ListSessions returns all sessions.
func (w *LLMWorkspace) ListSessions() ([]LLMSession, error) {
	idx, err := w.readSessionIndex()
	if err != nil {
		return nil, err
	}

	return idx.Sessions, nil
}

// CurrentSession returns the current active session.
func (w *LLMWorkspace) CurrentSession() (*LLMSession, error) {
	idx, err := w.readSessionIndex()
	if err != nil {
		return nil, err
	}

	if idx.Current == "" {
		return nil, fmt.Errorf("scout: workspace: no current session")
	}

	return w.GetSession(idx.Current)
}

// SetCurrentSession sets the current active session by ID.
func (w *LLMWorkspace) SetCurrentSession(id string) error {
	idx, err := w.readSessionIndex()
	if err != nil {
		return err
	}

	found := false
	for _, s := range idx.Sessions {
		if s.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("scout: workspace: session %q not found", id)
	}

	idx.Current = id
	return w.writeJSON(w.sessionsPath(), idx)
}

// CreateJob creates a new job in the given session.
func (w *LLMWorkspace) CreateJob(sessionID, url, prompt string, meta map[string]string) (*LLMJob, error) {
	jobID := uuid.New().String()
	now := time.Now().UTC()

	job := &LLMJob{
		ID:        jobID,
		SessionID: sessionID,
		Status:    JobStatusPending,
		URL:       url,
		Prompt:    prompt,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  meta,
	}

	// Create job directory
	jobDir := filepath.Join(w.root, "jobs", jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return nil, fmt.Errorf("scout: workspace: create job dir: %w", err)
	}

	// Write job.json
	if err := w.writeJSON(filepath.Join(jobDir, "job.json"), job); err != nil {
		return nil, err
	}

	// Update jobs index
	idx, err := w.readJobIndex()
	if err != nil {
		return nil, err
	}

	idx.Jobs = append(idx.Jobs, JobRef{
		ID:        jobID,
		SessionID: sessionID,
		Status:    JobStatusPending,
		URL:       url,
		CreatedAt: now,
	})
	idx.Current = jobID

	if err := w.writeJSON(w.jobsIndexPath(), idx); err != nil {
		return nil, err
	}

	return job, nil
}

// UpdateJob writes the updated job state to disk and syncs the index.
func (w *LLMWorkspace) UpdateJob(job *LLMJob) error {
	job.UpdatedAt = time.Now().UTC()

	jobDir := filepath.Join(w.root, "jobs", job.ID)
	if err := w.writeJSON(filepath.Join(jobDir, "job.json"), job); err != nil {
		return err
	}

	// Write extract/review results to separate files for easy reading
	if job.ExtractResult != "" {
		_ = os.WriteFile(filepath.Join(jobDir, "extract.md"), []byte(job.ExtractResult), 0o644)
	}
	if job.ReviewResult != "" {
		_ = os.WriteFile(filepath.Join(jobDir, "review.md"), []byte(job.ReviewResult), 0o644)
	}

	// Sync status in index
	idx, err := w.readJobIndex()
	if err != nil {
		return err
	}

	for i := range idx.Jobs {
		if idx.Jobs[i].ID == job.ID {
			idx.Jobs[i].Status = job.Status
			break
		}
	}

	return w.writeJSON(w.jobsIndexPath(), idx)
}

// GetJob reads a job by ID from its job directory.
func (w *LLMWorkspace) GetJob(id string) (*LLMJob, error) {
	jobPath := filepath.Join(w.root, "jobs", id, "job.json")

	var job LLMJob
	if err := w.readJSON(jobPath, &job); err != nil {
		return nil, fmt.Errorf("scout: workspace: job %q not found: %w", id, err)
	}

	return &job, nil
}

// ListJobs returns all job references from the index.
func (w *LLMWorkspace) ListJobs() ([]JobRef, error) {
	idx, err := w.readJobIndex()
	if err != nil {
		return nil, err
	}

	return idx.Jobs, nil
}

// CurrentJob returns the current active job.
func (w *LLMWorkspace) CurrentJob() (*LLMJob, error) {
	idx, err := w.readJobIndex()
	if err != nil {
		return nil, err
	}

	if idx.Current == "" {
		return nil, fmt.Errorf("scout: workspace: no current job")
	}

	return w.GetJob(idx.Current)
}

// ListSessionJobs returns all jobs for a specific session.
func (w *LLMWorkspace) ListSessionJobs(sessionID string) ([]JobRef, error) {
	idx, err := w.readJobIndex()
	if err != nil {
		return nil, err
	}

	var refs []JobRef
	for _, r := range idx.Jobs {
		if r.SessionID == sessionID {
			refs = append(refs, r)
		}
	}

	return refs, nil
}

// --- internal helpers ---

func (w *LLMWorkspace) sessionsPath() string {
	return filepath.Join(w.root, "sessions.json")
}

func (w *LLMWorkspace) jobsIndexPath() string {
	return filepath.Join(w.root, "jobs", "jobs.json")
}

func (w *LLMWorkspace) readSessionIndex() (*SessionIndex, error) {
	var idx SessionIndex
	if err := w.readJSON(w.sessionsPath(), &idx); err != nil {
		return nil, fmt.Errorf("scout: workspace: read sessions: %w", err)
	}
	return &idx, nil
}

func (w *LLMWorkspace) readJobIndex() (*JobIndex, error) {
	var idx JobIndex
	if err := w.readJSON(w.jobsIndexPath(), &idx); err != nil {
		return nil, fmt.Errorf("scout: workspace: read jobs index: %w", err)
	}
	return &idx, nil
}

func (w *LLMWorkspace) readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (w *LLMWorkspace) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: workspace: marshal JSON: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}
