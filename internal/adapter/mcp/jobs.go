package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// JobSnapshot is a thread-safe, mutex-free snapshot of Job for serialization.
type JobSnapshot struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"` // "run" | "update"
	Status     JobStatus        `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at,omitempty"`
	Events     []ucrun.RunEvent `json:"events,omitempty"`
	Result     any              `json:"result,omitempty"`
	Err        string           `json:"error,omitempty"`
}

// Job represents an asynchronous task running in the MCP server.
type Job struct {
	ID         string
	Kind       string
	Status     JobStatus
	StartedAt  time.Time
	FinishedAt time.Time
	Events     []ucrun.RunEvent
	Result     any
	Err        string

	cancel context.CancelFunc
	mu     sync.RWMutex
}

// Snapshot returns a copy of the job data safe for JSON serialization.
func (j *Job) Snapshot() JobSnapshot {
	j.mu.RLock()
	defer j.mu.RUnlock()

	eventsCopy := make([]ucrun.RunEvent, len(j.Events))
	copy(eventsCopy, j.Events)

	return JobSnapshot{
		ID:         j.ID,
		Kind:       j.Kind,
		Status:     j.Status,
		StartedAt:  j.StartedAt,
		FinishedAt: j.FinishedAt,
		Events:     eventsCopy,
		Result:     j.Result,
		Err:        j.Err,
	}
}

// AddEvent appends a runtime event to the job safely.
func (j *Job) AddEvent(evt ucrun.RunEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Events = append(j.Events, evt)
}

// Finish sets the terminal state and result of a job.
func (j *Job) Finish(status JobStatus, result any, errMsg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	j.FinishedAt = time.Now()
	j.Result = result
	j.Err = errMsg
}

// Cancel requests job cancellation if still running.
func (j *Job) Cancel() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status == JobQueued || j.Status == JobRunning {
		j.Status = JobCancelled
		j.FinishedAt = time.Now()
		if j.cancel != nil {
			j.cancel()
		}
		return true
	}
	return false
}

// JobManager manages background asynchronous jobs with concurrency keys.
type JobManager struct {
	jobs       map[string]*Job
	jobsMu     sync.RWMutex
	dirLocks   map[string]*sync.Mutex
	dirLocksMu sync.Mutex
}

// NewJobManager initializes a clean JobManager.
func NewJobManager() *JobManager {
	return &JobManager{
		jobs:     make(map[string]*Job),
		dirLocks: make(map[string]*sync.Mutex),
	}
}

// LockKey acquires a keyed mutex for serialized directory updates.
func (m *JobManager) LockKey(key string) func() {
	m.dirLocksMu.Lock()
	l, ok := m.dirLocks[key]
	if !ok {
		l = &sync.Mutex{}
		m.dirLocks[key] = l
	}
	m.dirLocksMu.Unlock()

	l.Lock()
	return func() {
		l.Unlock()
	}
}

// CreateJob registers a new background job with an allocated ID and cancelable context.
func (m *JobManager) CreateJob(ctx context.Context, kind string) (context.Context, *Job) {
	jobCtx, cancel := context.WithCancel(ctx)

	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	id := hex.EncodeToString(buf)

	j := &Job{
		ID:        id,
		Kind:      kind,
		Status:    JobQueued,
		StartedAt: time.Now(),
		cancel:    cancel,
	}

	m.jobsMu.Lock()
	m.jobs[id] = j
	m.jobsMu.Unlock()

	return jobCtx, j
}

// GetJob returns a snapshot of a job by ID or false if not found.
func (m *JobManager) GetJob(id string) (JobSnapshot, bool) {
	m.jobsMu.RLock()
	defer m.jobsMu.RUnlock()
	j, ok := m.jobs[id]
	if !ok {
		return JobSnapshot{}, false
	}
	return j.Snapshot(), true
}

// CancelJob attempts to cancel a job by ID.
func (m *JobManager) CancelJob(id string) bool {
	m.jobsMu.RLock()
	j, ok := m.jobs[id]
	m.jobsMu.RUnlock()
	if !ok {
		return false
	}
	return j.Cancel()
}
