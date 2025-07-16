package gui

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WordJob represents a single word processing job
type WordJob struct {
	ID           int
	Word         string
	Translation  string
	AudioFile    string
	ImageFiles   []string
	Status       JobStatus
	Error        error
	StartedAt    time.Time
	CompletedAt  time.Time
	CustomPrompt string // Custom prompt for image generation
}

// JobStatus represents the current state of a job
type JobStatus int

const (
	StatusQueued JobStatus = iota
	StatusProcessing
	StatusCompleted
	StatusFailed
)

func (s JobStatus) String() string {
	switch s {
	case StatusQueued:
		return "Queued"
	case StatusProcessing:
		return "Processing"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// WordQueue manages the queue of words to be processed
type WordQueue struct {
	jobs       chan *WordJob
	results    map[int]*WordJob
	processing map[int]*WordJob
	completed  []*WordJob
	
	nextID     int
	mu         sync.RWMutex
	
	// Callbacks for UI updates
	onStatusUpdate func(job *WordJob)
	onJobComplete  func(job *WordJob)
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWordQueue creates a new word processing queue
func NewWordQueue(ctx context.Context) *WordQueue {
	queueCtx, cancel := context.WithCancel(ctx)
	
	q := &WordQueue{
		jobs:       make(chan *WordJob, 100),
		results:    make(map[int]*WordJob),
		processing: make(map[int]*WordJob),
		completed:  make([]*WordJob, 0),
		nextID:     1,
		ctx:        queueCtx,
		cancel:     cancel,
	}
	
	// Don't start a worker - the GUI will pull jobs
	
	return q
}

// SetCallbacks sets the callback functions for UI updates
func (q *WordQueue) SetCallbacks(onStatusUpdate func(*WordJob), onJobComplete func(*WordJob)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onStatusUpdate = onStatusUpdate
	q.onJobComplete = onJobComplete
}

// AddWord adds a word to the processing queue
func (q *WordQueue) AddWord(word string) *WordJob {
	return q.AddWordWithPrompt(word, "")
}

// AddWordWithPrompt adds a word to the processing queue with a custom prompt
func (q *WordQueue) AddWordWithPrompt(word, customPrompt string) *WordJob {
	q.mu.Lock()
	job := &WordJob{
		ID:           q.nextID,
		Word:         word,
		Status:       StatusQueued,
		CustomPrompt: customPrompt,
	}
	q.nextID++
	q.results[job.ID] = job
	q.mu.Unlock()
	
	// Try to add to queue
	select {
	case q.jobs <- job:
		q.updateJobStatus(job, StatusQueued)
		return job
	case <-q.ctx.Done():
		job.Status = StatusFailed
		job.Error = fmt.Errorf("queue is shutting down")
		return job
	}
}

// GetJob returns a job by ID
func (q *WordQueue) GetJob(id int) *WordJob {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.results[id]
}

// GetQueueStatus returns the current queue statistics
func (q *WordQueue) GetQueueStatus() (queued, processing, completed, failed int) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	// Count based on job statuses for accuracy
	for _, job := range q.results {
		switch job.Status {
		case StatusQueued:
			queued++
		case StatusProcessing:
			processing++
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		}
	}
	
	return
}

// GetActiveJobs returns all jobs that are currently queued or processing
func (q *WordQueue) GetActiveJobs() []*WordJob {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	var jobs []*WordJob
	
	// Add processing jobs
	for _, job := range q.processing {
		jobs = append(jobs, job)
	}
	
	// Add queued jobs from channel (non-blocking)
	queuedJobs := make([]*WordJob, 0)
	for {
		select {
		case job := <-q.jobs:
			queuedJobs = append(queuedJobs, job)
		default:
			// Re-add jobs back to queue
			for _, job := range queuedJobs {
				q.jobs <- job
			}
			jobs = append(jobs, queuedJobs...)
			return jobs
		}
	}
}

// GetCompletedJobs returns all completed jobs
func (q *WordQueue) GetCompletedJobs() []*WordJob {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return append([]*WordJob{}, q.completed...)
}

// Stop gracefully shuts down the queue
func (q *WordQueue) Stop() {
	q.cancel()
	close(q.jobs)
}

// CompleteJob marks a job as completed with results
func (q *WordQueue) CompleteJob(jobID int, translation, audioFile string, imageFiles []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if job, exists := q.results[jobID]; exists {
		job.Status = StatusCompleted
		job.Translation = translation
		job.AudioFile = audioFile
		job.ImageFiles = imageFiles
		job.CompletedAt = time.Now()
		
		delete(q.processing, jobID)
		q.completed = append(q.completed, job)
		
		if q.onJobComplete != nil {
			q.onJobComplete(job)
		}
	}
}

// FailJob marks a job as failed with an error
func (q *WordQueue) FailJob(jobID int, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if job, exists := q.results[jobID]; exists {
		job.Status = StatusFailed
		job.Error = err
		job.CompletedAt = time.Now()
		
		delete(q.processing, jobID)
		
		if q.onJobComplete != nil {
			q.onJobComplete(job)
		}
	}
}

// updateJobStatus updates the status of a job and calls the callback
func (q *WordQueue) updateJobStatus(job *WordJob, status JobStatus) {
	job.Status = status
	if q.onStatusUpdate != nil {
		q.onStatusUpdate(job)
	}
}

// ProcessNextJob should be called by the GUI to process the next job in queue
func (q *WordQueue) ProcessNextJob() *WordJob {
	select {
	case job := <-q.jobs:
		q.mu.Lock()
		q.processing[job.ID] = job
		job.Status = StatusProcessing
		job.StartedAt = time.Now()
		q.mu.Unlock()
		
		// Call the status update callback
		if q.onStatusUpdate != nil {
			q.onStatusUpdate(job)
		}
		
		return job
		
	default:
		return nil
	}
}

// RemoveCompletedJobByWord removes a completed job for a specific word
func (q *WordQueue) RemoveCompletedJobByWord(word string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	// Remove from completed jobs list
	newCompleted := make([]*WordJob, 0, len(q.completed))
	for _, job := range q.completed {
		if job.Word != word {
			newCompleted = append(newCompleted, job)
		}
	}
	q.completed = newCompleted
	
	// Also remove from results map
	for id, job := range q.results {
		if job.Word == word && job.Status == StatusCompleted {
			delete(q.results, id)
		}
	}
}