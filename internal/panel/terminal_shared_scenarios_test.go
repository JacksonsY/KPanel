package panel

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

// These are output transport consumers, not simulations of their business jobs.
var sharedOutputScenarios = []struct{ name, job string }{
	{"host", ""}, {"shortcut", ""}, {"docker", ""}, {"file-execution", ""},
	{"system-packages", ""}, {"site-management", ""}, {"app-install", "app"},
	{"app-script", "app"}, {"site-install", "site"}, {"diagnostic", "diagnostic"}, {"environment", "environment"},
}

type sharedOutputAgent struct {
	terminalAgentStub
	delay    time.Duration
	probes   atomic.Int32
	writes   atomic.Int32
	reads    atomic.Int32
	idle     bool
	recovery bool
}

func (a *sharedOutputAgent) Do(context.Context, string, string, string, string, []byte) (AgentResponse, error) {
	a.writes.Add(1)
	return AgentResponse{}, fmt.Errorf("output viewer must not mutate a job or session")
}

func (a *sharedOutputAgent) Get(ctx context.Context, path, query, _ string) (AgentResponse, error) {
	call := a.reads.Add(1)
	if a.recovery {
		if call == 2 {
			return AgentResponse{}, fmt.Errorf("transient output failure")
		}
		if call >= 4 {
			<-ctx.Done()
			return AgentResponse{}, ctx.Err()
		}
	}
	values, _ := url.ParseQuery(query)
	if values.Get("wait") == "0" {
		a.probes.Add(1)
	}
	if !a.idle && !a.recovery && values.Get("offset") != "0" && values.Get("wait") != "0" {
		<-ctx.Done()
		return AgentResponse{}, ctx.Err()
	}
	timer := time.NewTimer(a.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return AgentResponse{}, ctx.Err()
	case <-timer.C:
	}
	data := []byte("ready\r\n")
	if a.idle || a.recovery {
		body, _ := json.Marshal(jobTerminalChunk{NextOffset: 0, InputOpen: true})
		if strings.HasSuffix(path, "/output") {
			body = []byte(`{"offset":0,"nextOffset":0}`)
		}
		return AgentResponse{StatusCode: 200, Body: body}, nil
	}
	if values.Get("offset") != "0" {
		data = nil
	}
	var body []byte
	if strings.HasSuffix(path, "/output") {
		body, _ = json.Marshal(map[string]any{"data": data, "offset": 7 - len(data), "nextOffset": 7})
	} else {
		body, _ = json.Marshal(jobTerminalChunk{DataBase64: base64.StdEncoding.EncodeToString(data), NextOffset: 7, InputOpen: true})
	}
	return AgentResponse{StatusCode: http.StatusOK, Body: body}, nil
}

func TestSharedTerminalIdlePerformance(t *testing.T) {
	if os.Getenv("KPANEL_SHARED_TERMINAL_BENCH") != "1" {
		t.Skip("opt-in idle output comparison")
	}
	for _, job := range []string{"", "app", "site", "diagnostic", "environment"} {
		t.Run("job="+job, func(t *testing.T) {
			a := &sharedOutputAgent{idle: true}
			s := &Server{agent: a, terminalSessions: map[string]panelTerminalSession{"session": {ID: "session", UserID: "owner", HostID: cluster.LocalHostID}}}
			s.hostOps = newHostOperationService(s)
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			stream := &terminalStream{ctx: ctx, userID: "owner", events: make(chan terminalStreamEvent, 4)}
			if job == "" {
				s.pumpHostTerminal(ctx, stream, "key", terminalStreamSubscription{ID: "session"})
			} else {
				s.pumpJobTerminal(ctx, stream, "key", terminalStreamSubscription{ID: "job", Job: job})
			}
			t.Logf("observation=250ms ignored_long_poll=true reads=%d mutations=%d", a.reads.Load(), a.writes.Load())
			if os.Getenv("KPANEL_SHARED_EXPECT_BOUNDED") == "1" && a.reads.Load() > 5 {
				t.Fatal("idle backend is busy looping")
			}
		})
	}
}

func TestJobTerminalRecoveryWithoutNewOutput(t *testing.T) {
	for _, job := range []string{"app", "site", "diagnostic", "environment"} {
		t.Run(job, func(t *testing.T) {
			a := &sharedOutputAgent{recovery: true}
			s := &Server{agent: a}
			s.hostOps = newHostOperationService(s)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			stream := &terminalStream{ctx: ctx, events: make(chan terminalStreamEvent, 4)}
			done := make(chan struct{})
			go func() {
				defer close(done)
				s.pumpJobTerminal(ctx, stream, "job", terminalStreamSubscription{ID: "id", Job: job})
			}()
			for i := 0; i < 3; i++ {
				select {
				case e := <-stream.events:
					if i == 1 && e.Error != "terminal_output_failed" || i != 1 && e.Job == nil {
						t.Fatalf("event %d: %#v", i, e)
					}
				case <-ctx.Done():
					t.Fatal("empty recovery did not notify viewer")
				}
			}
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("pump survived cancellation")
			}
			if a.writes.Load() != 0 {
				t.Fatal("viewer mutated task")
			}
		})
	}
}

func TestTerminalSubscriptionsCapacityIsAtomic(t *testing.T) {
	s, tokenPath := newTestServer(t)
	cookie, csrf := bootstrapCookies(t, s, tokenPath)
	session, err := s.auth.Authenticate(cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &terminalStream{id: "atomic", ctx: ctx, cancel: cancel, userID: session.User.ID, tokenHash: sha256.Sum256([]byte(cookie.Value)), subs: map[string]context.CancelFunc{}}
	var cancelled atomic.Int32
	for i := 0; i < maxTerminalStreamSubscriptions; i++ {
		stream.subs[fmt.Sprintf("job:app:%032x", i)] = func() { cancelled.Add(1) }
	}
	s.terminalStreams.mu.Lock()
	s.terminalStreams.streams[stream.id] = stream
	s.terminalStreams.mu.Unlock()
	body := fmt.Sprintf(`{"streamId":"atomic","remove":["job:app:%032x"],"add":[{"kind":"job","job":"app","id":"%032x"},{"kind":"job","job":"app","id":"%032x"}]}`, 0, 100, 101)
	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, terminalStreamSubscriptionsPath, strings.NewReader(body))
		r.Header.Set("Origin", "http://panel.test")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf.Value)
		r.AddCookie(cookie)
		r.AddCookie(csrf)
		w := httptest.NewRecorder()
		s.updateTerminalStreamSubscriptions(w, r, cookie.Value, session)
		return w
	}
	w := post(body)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	stream.mu.Lock()
	n := len(stream.subs)
	stream.mu.Unlock()
	if cancelled.Load() != 0 || n != maxTerminalStreamSubscriptions {
		t.Fatalf("rejected request mutated subscriptions: cancelled=%d slots=%d", cancelled.Load(), n)
	}
	duplicate := fmt.Sprintf(`{"streamId":"atomic","add":[{"kind":"job","job":"app","id":"%032x"},{"kind":"job","job":"app","id":"%032x"}]}`, 0, 0)
	if response := post(duplicate); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate subscriptions: %d", response.Code)
	}
	cancel()
	if response := post(`{"streamId":"atomic","add":[],"remove":[]}`); response.Code != http.StatusNotFound {
		t.Fatalf("closed stream: %d", response.Code)
	}
	if cancelled.Load() != 0 {
		t.Fatal("rejected update stopped existing subscribers")
	}
}

func TestSharedTerminalBurstStillCoalesces(t *testing.T) {
	t.Run("common job coalescer", func(t *testing.T) {
		a := &jobChunkAgentStub{chunks: map[string]jobTerminalChunk{
			"0": {DataBase64: encodedJobData("a"), NextOffset: 1, InputOpen: true},
			"1": {DataBase64: encodedJobData("b"), NextOffset: 2, InputOpen: true},
			"2": {DataBase64: encodedJobData("c"), NextOffset: 3, Finished: true},
		}}
		// The common coalescer must retain burst merging after an immediate first chunk.
		s := &Server{agent: a}
		s.hostOps = newHostOperationService(s)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stream := &terminalStream{ctx: ctx, events: make(chan terminalStreamEvent, 4)}
		s.pumpJobTerminal(ctx, stream, "key", terminalStreamSubscription{Job: "app", ID: "id"})
		for i, want := range []string{"a", "bc"} {
			select {
			case e := <-stream.events:
				if e.Job == nil {
					t.Fatalf("event %d: %#v", i, e)
				}
				data, _ := base64.StdEncoding.DecodeString(e.Job.DataBase64)
				if string(data) != want {
					t.Fatalf("event %d=%q want %q", i, data, want)
				}
			default:
				t.Fatal("missing burst event")
			}
		}
	})
}

func sharedFirstOutput(t *testing.T, job string, delay time.Duration) (time.Duration, int32) {
	t.Helper()
	a := &sharedOutputAgent{delay: delay}
	s := &Server{agent: a, terminalSessions: map[string]panelTerminalSession{
		"session": {ID: "session", UserID: "owner", HostID: cluster.LocalHostID, BackendSessionID: "backend"},
	}}
	s.hostOps = newHostOperationService(s)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream := &terminalStream{ctx: ctx, userID: "owner", events: make(chan terminalStreamEvent, 4)}
	done := make(chan struct{})
	start := time.Now()
	go func() {
		defer close(done)
		if job == "" {
			s.pumpHostTerminal(ctx, stream, "key", terminalStreamSubscription{ID: "session"})
		} else {
			s.pumpJobTerminal(ctx, stream, "key", terminalStreamSubscription{ID: "job", Job: job})
		}
	}()
	select {
	case event := <-stream.events:
		elapsed := time.Since(start)
		var data []byte
		if event.Output != nil {
			data = event.Output.Data
		} else if event.Job != nil {
			data, _ = base64.StdEncoding.DecodeString(event.Job.DataBase64)
		}
		if string(data) != "ready\r\n" || event.Error != "" {
			t.Fatalf("unexpected output: %#v %q", event, data)
		}
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cancelled output pump leaked")
		}
		if a.writes.Load() != 0 {
			t.Fatal("viewer mutated backend")
		}
		return elapsed, a.probes.Load()
	case <-ctx.Done():
		t.Fatal("first output timed out")
	}
	return 0, 0
}

func TestSharedTerminalSparseFirstOutput(t *testing.T) {
	for _, scenario := range sharedOutputScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			_, probes := sharedFirstOutput(t, scenario.job, 0)
			if probes != 0 {
				t.Fatalf("first output blocked on %d speculative Agent reads", probes)
			}
		})
	}
}

func TestSharedTerminalOutputPerformance(t *testing.T) {
	if os.Getenv("KPANEL_SHARED_TERMINAL_BENCH") != "1" {
		t.Skip("opt-in transport comparison")
	}
	for _, scenario := range sharedOutputScenarios {
		for _, delay := range []time.Duration{0, 5 * time.Millisecond, 25 * time.Millisecond} {
			t.Run(fmt.Sprintf("%s/%s", scenario.name, delay), func(t *testing.T) {
				samples := make([]time.Duration, 12)
				var probes int32
				for i := range samples {
					var n int32
					samples[i], n = sharedFirstOutput(t, scenario.job, delay)
					probes += n
				}
				sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
				t.Logf("n=12 agent_delay=%v first_output_p50=%v first_output_p95=%v speculative_reads=%d", delay, samples[5].Round(time.Microsecond), samples[11].Round(time.Microsecond), probes)
			})
		}
	}
}
