package cluster

import (
	"context"
	"errors"
	"github.com/flynn/noise"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"io"
	"strings"
	"testing"
	"time"
)

type independentNodeIdentityRemote struct {
	remoteV2API
	remoteV2FileAPI
	ctx   context.Context
	calls int
}

func (r *independentNodeIdentityRemote) OpenFileV2(ctx context.Context, _, _, _ string, _ noise.DHKey, _ []byte, _ time.Time, _ FederationFileOpenRequest) (io.ReadCloser, contract.FileTransferMetadata, error) {
	r.ctx = ctx
	r.calls++
	return io.NopCloser(strings.NewReader("data")), contract.FileTransferMetadata{}, nil
}
func TestIndependentHostIDPolicyControlsRemoteNodeSource(t *testing.T) {
	s := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	record := testHostRecordV2(1, time.Now().UTC())
	record.Scope = SummaryTerminalFilesScope
	record.ResourceVersion = hostResourceVersionV2(record)
	var err error
	record.CredentialFile, err = s.secretsV2.WriteHostCredential(record.ID, testHostCredentialV2(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.storeV2.AddHost(record); err != nil {
		t.Fatal(err)
	}
	if record.ID == record.RemoteNodeID {
		t.Fatal("fixture must use distinct host and node identities")
	}
	remote := &independentNodeIdentityRemote{}
	s.remoteV2 = remote
	body, _, err := s.OpenRemoteFileV2(context.Background(), record.RemoteNodeID, FederationFileOpenRequest{Path: "/home/source", ResourceVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	admitted := remote.ctx
	if _, err = s.UpdateHostRemoteAccess(record.ID, "initial", true, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-admitted.Done():
	case <-time.After(100 * time.Millisecond):
		t.Error("in-flight source transfer survived the Host.ID file denial")
	}
	body2, _, err := s.OpenRemoteFileV2(context.Background(), record.RemoteNodeID, FederationFileOpenRequest{Path: "/home/source", ResourceVersion: "v1"})
	if body2 != nil {
		defer body2.Close()
	}
	if !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Errorf("new source transfer bypassed Host.ID policy via RemoteNodeID: err=%v transportCalls=%d", err, remote.calls)
	}
}
