//go:build !windows

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func TestLightFileEncryptedSlowDownloadDoesNotBlockOtherRequest(t *testing.T) {
	var service *cluster.Service
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope cluster.FederationEnvelopeV2
		if json.NewDecoder(r.Body).Decode(&envelope) != nil {
			http.Error(w, "invalid", http.StatusBadRequest)
			return
		}
		response, err := service.HandleFederationV2(r.Context(), "198.51.100.10", r.URL.Path, "", envelope)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	var err error
	service, err = cluster.NewService(cluster.ServiceConfig{DataDir: t.TempDir(), PublicURL: server.URL, PanelVersion: "1.14.1", Telemetry: fileRelayTestTelemetry{}})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	key, err := cluster.GenerateFederationV2Keypair()
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := service.CreateLightEnrollment()
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(enrollment.Command)
	identity, err := service.EnrollLightNode("198.51.100.10", cluster.LightEnrollRequest{Token: strings.Trim(fields[len(fields)-1], "'"), Name: "relay-test", NodeVersion: "1.14.1", TerminalPublicKey: base64.RawURLEncoding.EncodeToString(key.Public)})
	if err != nil {
		t.Fatal(err)
	}
	peer, err := base64.RawURLEncoding.DecodeString(identity.TerminalPeerPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	client, err := cluster.NewFileRelayClient(server.Client())
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("slow-download\n"), 50<<10)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/files/content" {
			_, _ = w.Write(payload)
			return
		}
		_, _ = io.WriteString(w, "directory")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		runLightFileControl(ctx, nodeConfig{Origin: server.URL, NodeID: identity.NodeID, TargetNodeID: identity.TargetNodeID}, terminalIdentity{Key: key, Peer: peer}, client, handler)
	}()
	defer func() { cancel(); <-finished }()
	for {
		host, err := service.Host(ctx, identity.NodeID)
		if err == nil && host.FileManagementAvailable {
			break
		}
		if !waitContext(ctx, 5*time.Millisecond) {
			t.Fatal("file relay did not connect")
		}
	}
	slow, err := service.OpenLightFile(ctx, identity.NodeID, cluster.LightFileRequest{Method: http.MethodGet, Path: "/v1/files/content", Body: http.NoBody})
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Body.Close()
	// The node has produced far more than the center's eight-chunk buffer.
	// Give the authenticated polling loop time to fill it while not reading.
	if !waitContext(ctx, 3*time.Second) {
		t.Fatal("timed out while filling buffer")
	}
	fastCtx, fastCancel := context.WithTimeout(ctx, 4*time.Second)
	defer fastCancel()
	fast, err := service.OpenLightFile(fastCtx, identity.NodeID, cluster.LightFileRequest{Method: http.MethodGet, Path: "/v1/files/entry", Body: http.NoBody})
	if err != nil {
		t.Fatalf("other request blocked by unread download: %v", err)
	}
	data, err := io.ReadAll(fast.Body)
	_ = fast.Body.Close()
	if err != nil || string(data) != "directory" {
		t.Fatalf("other response = %q, %v", data, err)
	}
	data, err = io.ReadAll(slow.Body)
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("resumed download changed: %d bytes, %v", len(data), err)
	}
}
