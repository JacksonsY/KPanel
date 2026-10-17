package cluster

// The v1 relay predates end-to-end request body authentication. Framed bodies
// authenticate bounded chunks before they reach the Agent, including an end
// record, their order and the unique signed HTTP request. No file-sized buffer,
// temporary copy or additional persisted credential is needed.
import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const fileBodyChunkBytes = 64 << 10
const fileBodyMaxBytes = 512 << 20

type fileBodyContextKey struct{}

func (c *RemoteClient) OpenBodySignedFileRelayV1(ctx context.Context, origin, controllerID, targetID string, key ed25519.PrivateKey, now time.Time, input LightFileRequest) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return c.openFileRelayV1(context.WithValue(ctx, fileBodyContextKey{}, true), origin, controllerID, targetID, key, now, input, true)
}

func fileBodyBinding(request *http.Request) string {
	headers := []string{}
	for _, name := range []string{"Accept", "Content-Type", "If-Modified-Since", "If-None-Match", "If-Range", "Range"} {
		headers = append(headers, name+":"+strconv.Quote(request.Header.Get(name)))
	}
	sort.Strings(headers)
	return "\n" + request.Header.Get(fileRelayV1BodyHeader) + "\n" + strings.Join(headers, "\n")
}

type signedFileBody struct {
	source   io.ReadCloser
	public   ed25519.PublicKey
	private  ed25519.PrivateKey
	binding  string
	sequence uint64
	total    int64
	pending  []byte
	ended    bool
	failure  error
}

func newSignedFileBody(source io.ReadCloser, public ed25519.PublicKey, private ed25519.PrivateKey, binding string) io.ReadCloser {
	return &signedFileBody{source: source, public: public, private: private, binding: binding}
}

func (b *signedFileBody) Close() error { return b.source.Close() }

func (b *signedFileBody) message(data []byte) []byte {
	digest := sha256.Sum256(data)
	return []byte("KPanel file body v1\n" + b.binding + "\n" + strconv.FormatUint(b.sequence, 10) + "\n" + strconv.Itoa(len(data)) + "\n" + hex.EncodeToString(digest[:]))
}

func (b *signedFileBody) Read(out []byte) (int, error) {
	if len(out) == 0 {
		return 0, nil
	}
	if b.failure != nil {
		return 0, b.failure
	}
	if len(b.pending) == 0 {
		if b.ended {
			return 0, io.EOF
		}
		if len(b.private) != 0 {
			b.failure = b.encode()
		} else {
			b.failure = b.decode()
		}
		if b.failure != nil {
			return 0, b.failure
		}
		if len(b.pending) == 0 && b.ended {
			return 0, io.EOF
		}
	}
	n := copy(out, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}

func (b *signedFileBody) encode() error {
	data := make([]byte, fileBodyChunkBytes)
	n, err := io.ReadFull(b.source, data)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	data = data[:n]
	b.total += int64(n)
	if b.total > fileBodyMaxBytes {
		return ErrAuthentication
	}
	b.pending = make([]byte, 4+n+ed25519.SignatureSize)
	binary.BigEndian.PutUint32(b.pending, uint32(n))
	copy(b.pending[4:], data)
	copy(b.pending[4+n:], ed25519.Sign(b.private, b.message(data)))
	b.sequence++
	if n == 0 {
		b.ended = true
	}
	return nil
}

func (b *signedFileBody) decode() error {
	var size [4]byte
	if _, err := io.ReadFull(b.source, size[:]); err != nil {
		return io.ErrUnexpectedEOF
	}
	n := int(binary.BigEndian.Uint32(size[:]))
	if n > fileBodyChunkBytes || b.total+int64(n) > fileBodyMaxBytes {
		return ErrAuthentication
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(b.source, data); err != nil {
		return io.ErrUnexpectedEOF
	}
	var signature [ed25519.SignatureSize]byte
	if _, err := io.ReadFull(b.source, signature[:]); err != nil {
		return io.ErrUnexpectedEOF
	}
	if !ed25519.Verify(b.public, b.message(data), signature[:]) {
		return ErrAuthentication
	}
	b.sequence++
	b.total += int64(n)
	if n == 0 {
		// Reject appended data too. HTTP body EOF bounds this read to this request.
		var extra [1]byte
		if count, err := b.source.Read(extra[:]); count != 0 || err != io.EOF {
			return ErrAuthentication
		}
		b.ended = true
		return nil
	}
	b.pending = data
	return nil
}
