package filemanager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

type receiveErrorReader struct{ err error }

func (r receiveErrorReader) Read(data []byte) (int, error) {
	copy(data, "bad")
	return min(3, len(data)), r.err
}

type receiveEmptyReader struct{}

func (receiveEmptyReader) Read([]byte) (int, error) { return 0, nil }

func TestReceiveChunkBufferBoundsAndPromotion(t *testing.T) {
	for _, size := range []int{0, 4096, 4097, 65536, 65537, contract.FileTransferChunkBytes, contract.FileTransferChunkBytes + 1} {
		input := bytes.Repeat([]byte{0xa7}, size)
		data, err := readReceiveChunk(context.Background(), bytes.NewReader(input))
		if size > contract.FileTransferChunkBytes {
			if !errors.Is(err, ErrTooLarge) {
				t.Fatalf("size=%d error=%v", size, err)
			}
			continue
		}
		if err != nil || !bytes.Equal(data, input) || cap(data) > contract.FileTransferChunkBytes {
			t.Fatalf("size=%d len/cap=%d/%d error=%v", size, len(data), cap(data), err)
		}
		if size <= 4096 && cap(data) != 4096 {
			t.Fatalf("small chunk unnecessarily promoted: %d", cap(data))
		}
	}
}

func TestReceiveChunkBufferPreservesTransportFailureAndCancellation(t *testing.T) {
	for _, failure := range []error{io.ErrUnexpectedEOF, context.Canceled} {
		if data, err := readReceiveChunk(context.Background(), receiveErrorReader{failure}); !errors.Is(err, failure) || data != nil {
			t.Fatalf("transport failure hidden: %x %v", data, err)
		}
	}
	if _, err := readReceiveChunk(context.Background(), receiveEmptyReader{}); !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("empty reader did not stop: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readReceiveChunk(ctx, bytes.NewBufferString("data")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read proceeded: %v", err)
	}
}
