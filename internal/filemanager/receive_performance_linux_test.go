package filemanager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

// Opt-in frozen-binary comparison; normal tests never write a benchmark corpus.
// Run each sample in a new bounded process on a task-owned real filesystem.
func TestReceivePerformanceSample(t *testing.T) {
	if os.Getenv("KPANEL_RECEIVE_PERF") != "1" {
		t.Skip("opt-in receive performance sample")
	}
	workers, err := strconv.Atoi(os.Getenv("KPANEL_RECEIVE_PERF_WORKERS"))
	if err != nil || workers < 1 || workers > 2 {
		t.Fatal("workers must be 1 or 2")
	}
	chunkSize, chunkCount := contract.FileTransferChunkBytes, 8
	if os.Getenv("KPANEL_RECEIVE_PERF_SMALL") == "1" {
		chunkSize, chunkCount = 4096, 1
	}
	root := t.TempDir()
	m := newReceiveManager(t, root)
	ctx := context.Background()
	controlInput := testReceiveInput(0)
	controlInput.Name, controlInput.SourceKey = "control.bin", strings.Repeat("c", 64)
	control, err := m.BeginReceive(ctx, controlInput)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]contract.FileReceiveInput, workers)
	sessions := make([]contract.FileReceiveSession, workers)
	data := make([][]byte, workers)
	for w := 0; w < workers; w++ {
		inputs[w] = testReceiveInput(int64(chunkSize * chunkCount))
		inputs[w].Name, inputs[w].SourceKey = fmt.Sprintf("result-%d.bin", w), strings.Repeat(fmt.Sprint(w+1), 64)
		sessions[w], err = m.BeginReceive(ctx, inputs[w])
		if err != nil {
			t.Fatal(err)
		}
		data[w] = bytes.Repeat([]byte{byte(w + 31)}, chunkSize)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var usageBefore, usageAfter syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usageBefore); err != nil {
		t.Fatal(err)
	}
	var durations []time.Duration
	var observerErr error
	observerCtx, stop := context.WithCancel(ctx)
	observed := make(chan struct{})
	go func() {
		defer close(observed)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-observerCtx.Done():
				return
			case <-ticker.C:
				checkCtx, cancel := context.WithTimeout(observerCtx, time.Second)
				started := time.Now()
				_, checkErr := m.ReceiveStatus(checkCtx, control.ID, controlInput.SourceKey)
				elapsed := time.Since(started)
				cancel()
				if observerCtx.Err() != nil {
					return
				}
				if checkErr != nil {
					observerErr = checkErr
					return
				}
				if len(durations) < 8192 {
					durations = append(durations, elapsed)
				}
			}
		}
	}()
	expected := make([]string, workers)
	errors := make(chan error, workers)
	var writers sync.WaitGroup
	started := time.Now()
	for w := 0; w < workers; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			session := sessions[w]
			hasher := sha256.New()
			for index := 0; index < chunkCount; index++ {
				// Different absolute offsets and writer IDs expose whole-chunk swaps.
				for pos := 0; pos < chunkSize; pos += 64 << 10 {
					binary.LittleEndian.PutUint64(data[w][pos:], uint64(index*chunkSize+pos))
					binary.LittleEndian.PutUint64(data[w][pos+8:], uint64(w))
				}
				var writeErr error
				session, writeErr = m.WriteReceiveChunk(ctx, session.ID, inputs[w].SourceKey, session.Offset, bytes.NewReader(data[w]), receiveDigest(data[w]))
				if writeErr != nil {
					errors <- writeErr
					return
				}
				_, _ = hasher.Write(data[w])
			}
			expected[w] = hex.EncodeToString(hasher.Sum(nil))
			_, commitErr := m.CommitReceive(ctx, session.ID, inputs[w].SourceKey, inputs[w].SizeBytes, expected[w])
			errors <- commitErr
		}(w)
	}
	writers.Wait()
	elapsed := time.Since(started)
	stop()
	<-observed
	runtime.ReadMemStats(&after)
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usageAfter); err != nil {
		t.Fatal(err)
	}
	for w := 0; w < workers; w++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	if observerErr != nil {
		t.Fatal(observerErr)
	}
	for w := 0; w < workers; w++ {
		file, err := os.Open(filepath.Join(root, inputs[w].Name))
		if err != nil {
			t.Fatal(err)
		}
		hasher := sha256.New()
		size, err := io.Copy(hasher, file)
		_ = file.Close()
		if err != nil || size != inputs[w].SizeBytes || hex.EncodeToString(hasher.Sum(nil)) != expected[w] {
			t.Fatal("complete file checksum/size differs", err)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	var p95 float64
	if len(durations) > 0 {
		p95 = float64(durations[(len(durations)-1)*95/100].Microseconds()) / 1000
	}
	result := struct {
		Workers               int     `json:"workers"`
		Bytes                 int64   `json:"bytes"`
		Milliseconds          float64 `json:"milliseconds"`
		AllocatedBytes        uint64  `json:"allocatedBytes"`
		StatusSamples         int     `json:"statusSamples"`
		StatusP95Milliseconds float64 `json:"statusP95Milliseconds"`
		CPUSeconds            float64 `json:"cpuSeconds"`
		PeakRSSBytes          int64   `json:"peakRSSBytes"`
	}{workers, int64(workers * chunkSize * chunkCount), float64(elapsed.Microseconds()) / 1000, after.TotalAlloc - before.TotalAlloc, len(durations), p95,
		float64(usageAfter.Utime.Nano()+usageAfter.Stime.Nano()-usageBefore.Utime.Nano()-usageBefore.Stime.Nano()) / 1e9, usageAfter.Maxrss * 1024}
	encoded, _ := json.Marshal(result)
	fmt.Println("RECEIVE_PERF", string(encoded))
}
