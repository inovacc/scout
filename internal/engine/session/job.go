package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/segmentio/ksuid"
)

const jobFile = "job.json"

// JobStatus represents the current state of a job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// JobStep represents a single step within a job.
type JobStep struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error,omitempty"`
}

// Progress tracks the current progress of a job.
type Progress struct {
	CurrentStep int     `json:"current_step"`
	TotalSteps  int     `json:"total_steps"`
	Percentage  float64 `json:"percentage"`
	Message     string  `json:"message,omitempty"`
}

// Job holds all metadata for a session job, stored as job.json
// inside the session's data directory.
type Job struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Status      JobStatus `json:"status"`
	TargetURLs  []string  `json:"target_urls,omitempty"`
	Command     string    `json:"command,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CompletedAt time.Time `json:"completed_at,omitzero"`
	Progress    *Progress `json:"progress,omitempty"`
	Steps       []JobStep `json:"steps,omitempty"`
	Error       string    `json:"error,omitempty"`
	Output      string    `json:"output,omitempty"`
}

// NewJob creates a new Job with a KSUID, pending status, and timestamps.
func NewJob(jobType string, targetURLs []string, command string) *Job {
	now := time.Now()

	return &Job{
		ID:         ksuid.New().String(),
		Type:       jobType,
		Status:     JobPending,
		TargetURLs: targetURLs,
		Command:    command,
		StartedAt:  now,
		UpdatedAt:  now,
	}
}

// WriteJob writes the job as JSON to <SessionsDir>/<sessionID>/job.json.
func WriteJob(sessionID string, job *Job) error {
	dir := Dir(sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, jobFile), data, 0o644)
}

// ReadJob reads the job from <SessionsDir>/<sessionID>/job.json.
func ReadJob(sessionID string) (*Job, error) {
	data, err := os.ReadFile(filepath.Join(Dir(sessionID), jobFile))
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("scout: session: %w", err)
	}

	return &job, nil
}

// RemoveJob removes the job.json file from a session directory.
func RemoveJob(sessionID string) error {
	return os.Remove(filepath.Join(Dir(sessionID), jobFile))
}

// StartJob transitions a job from pending to running.
func StartJob(sessionID string) error {
	job, err := ReadJob(sessionID)
	if err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	job.Status = JobRunning
	job.UpdatedAt = time.Now()

	return WriteJob(sessionID, job)
}

// UpdateJobProgress updates the progress fields on a job.
func UpdateJobProgress(sessionID string, current, total int, message string) error {
	job, err := ReadJob(sessionID)
	if err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	pct := 0.0
	if total > 0 {
		pct = float64(current) / float64(total) * 100
	}

	job.Progress = &Progress{
		CurrentStep: current,
		TotalSteps:  total,
		Percentage:  pct,
		Message:     message,
	}
	job.UpdatedAt = time.Now()

	return WriteJob(sessionID, job)
}

// AddJobStep appends a step to the job and auto-updates progress.
func AddJobStep(sessionID string, step JobStep) error {
	job, err := ReadJob(sessionID)
	if err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	job.Steps = append(job.Steps, step)

	total := len(job.Steps)
	if job.Progress != nil {
		total = max(job.Progress.TotalSteps, len(job.Steps))
	}

	pct := 0.0
	if total > 0 {
		pct = float64(len(job.Steps)) / float64(total) * 100
	}

	job.Progress = &Progress{
		CurrentStep: len(job.Steps),
		TotalSteps:  total,
		Percentage:  pct,
		Message:     step.Name,
	}
	job.UpdatedAt = time.Now()

	return WriteJob(sessionID, job)
}

// CompleteJob marks a job as completed with output and timestamp.
func CompleteJob(sessionID string, output string) error {
	job, err := ReadJob(sessionID)
	if err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	now := time.Now()
	job.Status = JobCompleted
	job.Output = output
	job.CompletedAt = now
	job.UpdatedAt = now

	return WriteJob(sessionID, job)
}

// FailJob marks a job as failed with an error message and timestamp.
func FailJob(sessionID string, errMsg string) error {
	job, err := ReadJob(sessionID)
	if err != nil {
		return fmt.Errorf("scout: session: %w", err)
	}

	now := time.Now()
	job.Status = JobFailed
	job.Error = errMsg
	job.CompletedAt = now
	job.UpdatedAt = now

	return WriteJob(sessionID, job)
}
