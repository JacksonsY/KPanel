package virtualmachine

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/atomicfile"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/jobcontrol"
	"github.com/kejilion/kejilion-panel/internal/redact"
	"golang.org/x/crypto/ssh"
)

type jobRecord struct {
	contract.VMJob
	Request contract.VMRequest `json:"request"`
}
type taskRunner struct{ Runner }

func (r taskRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.Runner.Run(ctx, nil, name, args...)
}

func NewID() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", randomBytes(4), randomBytes(2), randomBytes(2), randomBytes(2), randomBytes(6))
}
func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func terminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "needs_attention"
}
func (s *Service) spec(id string) jobcontrol.Spec {
	return jobcontrol.Spec{Unit: "kpanel-vm-" + id, Executable: s.executable, Arguments: []string{"vm-run", "--state-dir", s.dir, "--job", id}, StateDir: s.dir, UMask: "0077", Nice: 10, IOClass: "2:7", StopTimeout: 10 * time.Second, SystemdProperties: []string{"Type=exec", "RuntimeMaxSec=35min", "TimeoutStopSec=10s", "MemoryMax=1G", "TasksMax=128"}}
}

func (s *Service) readJob(id string) (jobRecord, error) {
	var j jobRecord
	if !contract.ValidVMID(id) {
		return j, ErrInvalid
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return j, err
	}
	defer root.Close()
	f, err := root.Open(id + ".json")
	if err != nil {
		return j, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<10 || info.Mode().Perm()&0077 != 0 || !rootOwned(info) {
		return j, errors.New("invalid task file")
	}
	d := json.NewDecoder(io.LimitReader(f, 32<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(&j); err != nil {
		return j, err
	}
	if j.ID != id || j.Request.RequestID != id {
		return j, errors.New("task identity mismatch")
	}
	return j, nil
}

func (s *Service) save(j *jobRecord) error {
	j.UpdatedAt = time.Now().UTC()
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	r, err := (atomicfile.Writer{}).WritePrivate(s.dir, filepath.Join(s.dir, j.ID+".json"), ".vm-job-*", b)
	if err != nil {
		return err
	}
	if r.DurabilityErr != nil {
		return r.DurabilityErr
	}
	return r.CleanupErr
}

func (s *Service) Jobs(ctx context.Context) ([]contract.VMJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs(ctx)
}

func (s *Service) jobs(ctx context.Context) ([]contract.VMJob, error) {
	result := []contract.VMJob{}
	dir, err := os.Open(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	// Bound even the directory scan: unexpected files cannot make a request
	// allocate an arbitrarily large list.
	names, err := dir.Readdirnames(256)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(names) == 256 {
		return nil, errors.New("task directory capacity exceeded")
	}
	for _, name := range names {
		if filepath.Ext(name) != ".json" {
			continue
		}
		id := name[:len(name)-5]
		if !contract.ValidVMID(id) {
			continue
		}
		j, err := s.readJob(id)
		if err != nil {
			return nil, err
		}
		if !terminal(j.Status) && time.Since(j.UpdatedAt) > 30*time.Second {
			state, e := jobcontrol.Inspect(ctx, taskRunner{s.runner}, s.spec(id))
			if e == nil && state.Known && !state.Running {
				j.Status = "needs_attention"
				j.Message = "worker_interrupted"
				if e = s.save(&j); e != nil {
					return nil, e
				}
			}
		}
		result = append(result, j.VMJob)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (s *Service) Start(ctx context.Context, r contract.VMRequest) (contract.VMJob, error) {
	if !s.enabled {
		return contract.VMJob{}, ErrDisabled
	}
	if field, detail := contract.ValidateVMRequest(&r); field != "" {
		return contract.VMJob{}, fmt.Errorf("%w: %s: %s", ErrInvalid, field, detail)
	}
	if r.Source == "cloud" {
		if _, _, _, rest, err := ssh.ParseAuthorizedKey([]byte(r.PublicKey)); err != nil || len(rest) > 0 {
			return contract.VMJob{}, fmt.Errorf("%w: invalid SSH public key", ErrInvalid)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := privateDirectory(s.dir); err != nil {
		return contract.VMJob{}, err
	}
	unlock, err := lockDirectory(s.dir, "submit.lock")
	if err != nil {
		return contract.VMJob{}, err
	}
	defer unlock()
	if existing, err := s.readJob(r.RequestID); err == nil {
		a, _ := json.Marshal(existing.Request)
		b, _ := json.Marshal(r)
		if string(a) != string(b) {
			return contract.VMJob{}, ErrConflict
		}
		return existing.VMJob, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return contract.VMJob{}, err
	}
	jobs, err := s.jobs(ctx)
	if err != nil {
		return contract.VMJob{}, err
	}
	for _, j := range jobs {
		if !terminal(j.Status) {
			return contract.VMJob{}, ErrBusy
		}
	}
	// A killed worker can leave an unlinked-to-any-domain download or temporary
	// cloud-init input. Clean only this service's temporary prefixes while the
	// cross-process worker lock proves no worker can still be using them.
	workerUnlock, err := lockDirectory(s.dir, "worker.lock")
	if err != nil {
		return contract.VMJob{}, err
	}
	directory, err := os.Open(s.dir)
	if err != nil {
		workerUnlock()
		return contract.VMJob{}, err
	}
	names, readErr := directory.Readdirnames(256)
	directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		workerUnlock()
		return contract.VMJob{}, readErr
	}
	for _, name := range names {
		if strings.HasPrefix(name, ".cloud-image-") || strings.HasPrefix(name, ".cloud-init-") {
			if err = os.Remove(filepath.Join(s.dir, name)); err != nil {
				workerUnlock()
				return contract.VMJob{}, err
			}
		}
	}
	workerUnlock()
	if len(jobs) >= 32 {
		for _, j := range jobs[24:] {
			if terminal(j.Status) {
				if err = os.Remove(filepath.Join(s.dir, j.ID+".json")); err != nil {
					return contract.VMJob{}, err
				}
			}
		}
	}
	if r.Action != "install" {
		if e := s.Environment(ctx); (r.Action == "create" && !e.Ready) || !e.Connected {
			return contract.VMJob{}, ErrUnsupported
		}
		if r.Action != "create" {
			m, err := s.Machine(ctx, r.ID)
			if err != nil {
				return contract.VMJob{}, err
			}
			if m.ResourceVersion != r.ExpectedResourceVersion {
				return contract.VMJob{}, ErrConflict
			}
		}
	}
	if err = jobcontrol.Available(taskRunner{s.runner}); err != nil {
		return contract.VMJob{}, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	j := jobRecord{VMJob: contract.VMJob{ID: r.RequestID, Action: r.Action, Name: r.Name, MachineID: r.ID, Status: "queued", Stage: "queued", CreatedAt: time.Now().UTC()}, Request: r}
	if r.Action == "create" {
		j.MachineID = NewID()
	}
	if err = s.save(&j); err != nil {
		return contract.VMJob{}, err
	}
	launchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err = jobcontrol.Launch(launchCtx, taskRunner{s.runner}, s.spec(j.ID)); err != nil {
		// Launch might have reached the init manager. Never retry automatically
		// or overwrite a worker's result after an uncertain acknowledgement.
		return j.VMJob, fmt.Errorf("task launch unconfirmed; refresh task %s: %w", j.ID, err)
	}
	return j.VMJob, nil
}

func (s *Service) RunJob(ctx context.Context, id string) error {
	if err := privateDirectory(s.dir); err != nil {
		return err
	}
	unlock, err := lockDirectory(s.dir, "worker.lock")
	if err != nil {
		return err
	}
	defer unlock()
	j, err := s.readJob(id)
	if err != nil {
		return err
	}
	if j.Status != "queued" {
		return ErrConflict
	}
	if field, _ := contract.ValidateVMRequest(&j.Request); field != "" {
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	j.Status = "running"
	if err = s.save(&j); err != nil {
		return err
	}
	stage := func(stage string) error { j.Stage = stage; return s.save(&j) }
	if j.Action == "install" {
		err = s.install(ctx, stage)
	} else if j.Action == "create" {
		err = s.create(ctx, &j, stage)
	} else {
		err = s.action(ctx, &j, stage)
	}
	if err != nil {
		j.Status = "needs_attention"
		j.Message = redact.Text(err.Error())
		if len(j.Message) > 1024 {
			j.Message = j.Message[:1024]
		}
	} else {
		j.Status = "succeeded"
		j.Stage = "completed"
		j.Message = ""
	}
	if writeErr := s.save(&j); writeErr != nil {
		return writeErr
	}
	return err
}
