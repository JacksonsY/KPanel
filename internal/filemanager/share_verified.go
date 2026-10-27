package filemanager

import (
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"math"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

const shareBlockSize int64 = 64 << 10

// Store one digest per block, not a copy of the potentially 10 GiB file.
type shareBlockHasher struct {
	hash   hash.Hash
	used   int64
	hashes [][sha256.Size]byte
}

func newShareBlockHasher(size int64) *shareBlockHasher {
	capacity := int64(0)
	if size > 0 && size <= contract.MaxFileShareBytes {
		capacity = (size + shareBlockSize - 1) / shareBlockSize
	}
	return &shareBlockHasher{hash: sha256.New(), hashes: make([][sha256.Size]byte, 0, capacity)}
}

func (h *shareBlockHasher) Write(p []byte) (int, error) {
	length := len(p)
	for len(p) > 0 {
		n := min(len(p), int(shareBlockSize-h.used))
		_, _ = h.hash.Write(p[:n])
		h.used += int64(n)
		p = p[n:]
		if h.used == shareBlockSize {
			h.appendHash()
		}
	}
	return length, nil
}

func (h *shareBlockHasher) appendHash() {
	var sum [sha256.Size]byte
	h.hash.Sum(sum[:0])
	h.hashes = append(h.hashes, sum)
	h.hash.Reset()
	h.used = 0
}

func (h *shareBlockHasher) finish() [][sha256.Size]byte {
	if h.used > 0 {
		h.appendHash()
	}
	return h.hashes
}

// A block is read into private memory and verified before any byte is exposed.
// Cached bytes remain safe even if the source is changed after verification.
type verifiedShareFile struct {
	source      io.ReadSeekCloser
	size        int64
	hashes      [][sha256.Size]byte
	position    int64
	cachedBlock int64
	cache       []byte
}

func (f *verifiedShareFile) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if f.position >= f.size {
		return 0, io.EOF
	}
	index := f.position / shareBlockSize
	if index != f.cachedBlock {
		if index >= int64(len(f.hashes)) {
			return 0, ErrConflict
		}
		start := index * shareBlockSize
		length := min(shareBlockSize, f.size-start)
		if cap(f.cache) < int(length) {
			f.cache = make([]byte, length)
		} else {
			f.cache = f.cache[:length]
		}
		// Invalidate before touching the buffer, including on read errors.
		f.cachedBlock = -1
		if _, err := f.source.Seek(start, io.SeekStart); err != nil {
			return 0, err
		}
		if _, err := io.ReadFull(f.source, f.cache); err != nil {
			return 0, ErrConflict
		}
		if sha256.Sum256(f.cache) != f.hashes[index] {
			return 0, ErrConflict
		}
		f.cachedBlock = index
	}
	offset := f.position % shareBlockSize
	n := copy(p, f.cache[offset:])
	f.position += int64(n)
	return n, nil
}

func (f *verifiedShareFile) Seek(offset int64, whence int) (int64, error) {
	var base int64
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = f.position
	case io.SeekEnd:
		base = f.size
	default:
		return 0, errors.New("invalid seek origin")
	}
	if offset < -base || (offset > 0 && base > math.MaxInt64-offset) {
		return 0, errors.New("invalid seek offset")
	}
	f.position = base + offset
	return f.position, nil
}

func (f *verifiedShareFile) Close() error { return f.source.Close() }
