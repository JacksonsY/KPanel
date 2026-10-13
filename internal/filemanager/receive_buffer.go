package filemanager

import (
	"context"
	"io"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

// Promote only after another byte actually exists: small files stay small and
// full chunks avoid ReadAll's repeated growth/copying and oversized final cap.
// One lookahead byte detects oversize without reserving a second full buffer.
func readReceiveChunk(ctx context.Context, source io.Reader) ([]byte, error) {
	r := &contextReader{ctx, source}
	data := make([]byte, 0, 4<<10)
	var lookahead [1]byte
	emptyReads := 0
	for {
		var n int
		var err error
		if len(data) == cap(data) {
			n, err = r.Read(lookahead[:])
			if n > 0 {
				if len(data) == contract.FileTransferChunkBytes {
					return nil, ErrTooLarge
				}
				nextCap := 64 << 10
				if cap(data) == nextCap {
					nextCap = contract.FileTransferChunkBytes
				}
				grown := make([]byte, len(data), nextCap)
				copy(grown, data)
				data = append(grown, lookahead[0])
			}
		} else {
			n, err = r.Read(data[len(data):cap(data)])
			data = data[:len(data)+n]
		}
		if err != nil {
			if err == io.EOF {
				return data, nil
			}
			return nil, err
		}
		if n == 0 {
			emptyReads++
			if emptyReads == 100 {
				return nil, io.ErrNoProgress
			}
		} else {
			emptyReads = 0
		}
	}
}
