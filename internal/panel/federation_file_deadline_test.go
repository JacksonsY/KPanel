package panel

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

type slowFederationAgent struct {
	*transferIntegrationAgent
	delay, openDelay time.Duration
}

func (a *slowFederationAgent) OpenStream(ctx context.Context, method, path, query, id string, body io.Reader, headers http.Header, size int64) (*http.Response, error) {
	response, err := a.transferIntegrationAgent.OpenStream(ctx, method, path, query, id, body, headers, size)
	if err == nil && path == "/v1/files/transfer/export" {
		if a.openDelay > 0 {
			timer := time.NewTimer(a.openDelay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				_ = response.Body.Close()
				return nil, ctx.Err()
			}
		}
		response.Body = &slowFederationBody{ReadCloser: response.Body, delay: a.delay}
	}
	return response, err
}

type slowFederationBody struct {
	io.ReadCloser
	delay time.Duration
}

func (b *slowFederationBody) Read(p []byte) (int, error) {
	if len(p) > 4096 {
		p = p[:4096]
	}
	time.Sleep(b.delay)
	return b.ReadCloser.Read(p)
}

// Exercise the real Panel federation endpoint and real Agent export. A short
// HTTP server deadline stands in for the deployed 60-second deadline; the
// source keeps making progress for longer than that deadline.
func federationFileExport(t *testing.T, delay, openDelay, writeTimeout, testTimeout time.Duration) ([]byte, []byte, time.Duration, error) {
	t.Helper()
	source, _ := newTestServer(t)
	agent, manager, root := realTransferAgent(t, source)
	source.agent = &slowFederationAgent{transferIntegrationAgent: agent, delay: delay, openDelay: openDelay}
	content := bytes.Repeat([]byte("0123456789abcdef"), 4096)
	if err := os.WriteFile(filepath.Join(root, "source.bin"), content, 0600); err != nil {
		t.Fatal(err)
	}
	entry, err := manager.Stat("/source.bin")
	if err != nil {
		t.Fatal(err)
	}
	network := httptest.NewUnstartedServer(source)
	network.Config.WriteTimeout = writeTimeout
	network.Start()
	t.Cleanup(network.Close)
	remote, err := cluster.NewRemoteClient(cluster.RemoteClientConfig{Dialer: func(ctx context.Context, networkName, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, networkName, network.Listener.Addr().String())
	}})
	if err != nil {
		t.Fatal(err)
	}
	center, err := cluster.NewService(cluster.ServiceConfig{DataDir: t.TempDir(), Telemetry: historyTestTelemetry{}, Remote: remote})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = center.Close() })
	code, err := source.cluster.CreatePairingCodeV2()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	_, port, _ := net.SplitHostPort(network.Listener.Addr().String())
	host, err := center.AddHost(ctx, cluster.AddHostInput{Origin: "http://8.8.8.8:" + port, PairingCode: code.Code})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	body, _, err := center.OpenRemoteFileV2(ctx, host.RemoteNodeID, cluster.FederationFileOpenRequest{Path: entry.Path, ResourceVersion: entry.ResourceVersion})
	if err != nil {
		return nil, content, time.Since(started), err
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	return got, content, time.Since(started), err
}

func TestFederationFileExportContinuesPastServerWriteTimeout(t *testing.T) {
	delay, writeTimeout, testTimeout := 40*time.Millisecond, 150*time.Millisecond, 8*time.Second
	if os.Getenv("KPANEL_FEDERATION_DEADLINE_SOAK") == "1" {
		delay, writeTimeout, testTimeout = 4200*time.Millisecond, 60*time.Second, 90*time.Second
	}
	got, content, elapsed, err := federationFileExport(t, delay, 0, writeTimeout, testTimeout)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("federated source truncated after %v: read=%d want=%d error=%v", elapsed, len(got), len(content), err)
	}
	if elapsed <= writeTimeout {
		t.Fatal("transfer did not exceed the server's absolute write timeout")
	}
}

func TestFederationFileExportWaitsForBoundedSourcePreparation(t *testing.T) {
	got, content, elapsed, err := federationFileExport(t, 0, 3500*time.Millisecond, 10*time.Second, 8*time.Second)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("source preparation failed after %v: read=%d want=%d error=%v", elapsed, len(got), len(content), err)
	}
}

func TestFederationFileExportCancelsSourcePreparation(t *testing.T) {
	_, _, elapsed, err := federationFileExport(t, 0, 3500*time.Millisecond, 10*time.Second, 500*time.Millisecond)
	if err == nil || elapsed > 2*time.Second {
		t.Fatalf("canceled source preparation did not stop promptly: elapsed=%v error=%v", elapsed, err)
	}
}
