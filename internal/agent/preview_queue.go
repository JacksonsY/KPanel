package agent

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/filemanager"
)

const previewQueueLimit = 16

var errPreviewQueueFull = errors.New("preview queue is full")

type previewJob struct {
	entry      contract.FileEntry
	key        string
	foreground bool
	done       chan struct{}
	body       []byte
	mime       string
	err        error
}

type previewQueue struct {
	mu         sync.Mutex
	server     *Server
	disk       *previewDisk
	jobs       map[string]*previewJob
	queued     []*previewJob
	running    bool
	workerDone chan struct{}
	quietUntil time.Time
	wake       chan struct{}
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
}

func newPreviewQueue(s *Server, stateDir string) *previewQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &previewQueue{server: s, disk: newPreviewDisk(stateDir), jobs: make(map[string]*previewJob), ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
}
func screenPreviewKey(entry contract.FileEntry) string {
	return "preview\x00" + thumbnailCacheKey(entry.Path, entry.ResourceVersion)
}

func (q *previewQueue) submit(entry contract.FileEntry, foreground bool) (*previewJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, context.Canceled
	}
	if foreground {
		select {
		case q.wake <- struct{}{}:
		default:
		}
	}
	key := screenPreviewKey(entry)
	if job := q.jobs[key]; job != nil {
		job.foreground = job.foreground || foreground
		return job, nil
	}
	if len(q.jobs) >= previewQueueLimit {
		// User-opened images may displace a queued optional warmup, never active work.
		victim := -1
		if foreground {
			for i := len(q.queued) - 1; i >= 0; i-- {
				if !q.queued[i].foreground {
					victim = i
					break
				}
			}
		}
		if victim < 0 {
			return nil, errPreviewQueueFull
		}
		job := q.queued[victim]
		q.queued = append(q.queued[:victim], q.queued[victim+1:]...)
		delete(q.jobs, job.key)
		job.err = errPreviewQueueFull
		close(job.done)
	}
	job := &previewJob{entry: entry, key: key, foreground: foreground, done: make(chan struct{})}
	q.jobs[key] = job
	q.queued = append(q.queued, job)
	if !q.running {
		q.running = true
		q.workerDone = make(chan struct{})
		go q.run(q.workerDone)
	}
	return job, nil
}

func (q *previewQueue) get(ctx context.Context, entry contract.FileEntry) ([]byte, string, error) {
	if body, mime, ok := q.server.previews.get(screenPreviewKey(entry)); ok {
		return body, mime, nil
	}
	// Disk hits do not wait behind decoding or the work queue.
	if body, mime, ok := q.disk.get(screenPreviewKey(entry)); ok {
		q.server.previews.put(screenPreviewKey(entry), body, mime)
		return body, mime, nil
	}
	job, err := q.submit(entry, true)
	if err != nil {
		return nil, "", err
	}
	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	case <-job.done:
		return job.body, job.mime, job.err
	}
}

func (q *previewQueue) warm(entry contract.FileEntry) {
	if entry.SizeBytes <= 256<<10 || entry.SizeBytes > fileThumbnailMaxBytes || (entry.MIME != "image/jpeg" && entry.MIME != "image/png") {
		return
	}
	if _, _, ok := q.server.previews.get(screenPreviewKey(entry)); ok {
		return
	}
	// Give the initial grid a quiet CPU window; new thumbnails extend it.
	q.mu.Lock()
	q.quietUntil = time.Now().Add(250 * time.Millisecond)
	q.mu.Unlock()
	_, _ = q.submit(entry, false)
}

func (q *previewQueue) run(done chan struct{}) {
	defer close(done)
	for {
		q.mu.Lock()
		if len(q.queued) == 0 || q.closed {
			q.running = false
			q.mu.Unlock()
			return
		}
		index := 0
		for i, job := range q.queued {
			if job.foreground {
				index = i
				break
			}
		}
		if !q.queued[index].foreground && time.Now().Before(q.quietUntil) {
			delay := time.Until(q.quietUntil)
			q.mu.Unlock()
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-q.wake:
			case <-q.ctx.Done():
			}
			timer.Stop()
			continue
		}
		job := q.queued[index]
		q.queued = append(q.queued[:index], q.queued[index+1:]...)
		q.mu.Unlock()
		body, mime, err := q.generate(job)
		q.mu.Lock()
		job.body, job.mime, job.err = body, mime, err
		delete(q.jobs, job.key)
		close(job.done)
		q.mu.Unlock()
	}
}

func (q *previewQueue) generate(job *previewJob) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(q.ctx, fileThumbnailTimeout)
	defer cancel()
	// Reopen only when work starts: no queued source descriptors or download slots.
	file, entry, err := q.server.files.Open(ctx, job.entry.Path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	if entry.ResourceVersion != job.entry.ResourceVersion {
		return nil, "", filemanager.ErrConflict
	}
	if body, mime, ok := q.server.previews.get(job.key); ok {
		return body, mime, nil
	}
	if body, mime, ok := q.disk.get(job.key); ok {
		q.server.previews.put(job.key, body, mime)
		return body, mime, nil
	}
	select {
	case q.server.previewGate <- struct{}{}:
		defer func() { <-q.server.previewGate }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	select {
	case q.server.thumbnailGate <- struct{}{}:
		defer func() { <-q.server.thumbnailGate }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	body, mime, err := makeFilePreview(file, entry.SizeBytes)
	file.Close() // Release the source slot before version/permission revalidation.
	if err != nil {
		return nil, "", err
	}
	if err = ctx.Err(); err != nil {
		return nil, "", err
	}
	check, current, err := q.server.files.Open(ctx, entry.Path)
	if err != nil {
		return nil, "", err
	}
	check.Close()
	if current.ResourceVersion != entry.ResourceVersion {
		return nil, "", filemanager.ErrConflict
	}
	q.server.previews.put(job.key, body, mime)
	q.disk.put(job.key, body, mime)
	return body, mime, nil
}

func (q *previewQueue) close() {
	if q == nil {
		return
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.cancel()
	for _, job := range q.queued {
		delete(q.jobs, job.key)
		job.err = context.Canceled
		close(job.done)
	}
	q.queued = nil
	done := q.workerDone
	q.mu.Unlock()
	if done != nil {
		<-done
	}
	q.disk.close()
}
