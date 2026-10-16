package panel

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

type capabilityAgentStub struct {
	retainedAgentStub
	response AgentResponse
	err      error
}

func (a *capabilityAgentStub) Do(ctx context.Context, method, path, query, id string, body []byte) (AgentResponse, error) {
	if path == "/v1/terminals/capabilities/input-protocol" {
		return a.response, a.err
	}
	return a.retainedAgentStub.Do(ctx, method, path, query, id, body)
}

func TestRetainedAgentPreflightDoesNotCreatePTYOnMissingOrFailedCapability(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		err         error
		unsupported bool
	}{
		{name: "old route missing", status: 404, unsupported: true},
		{name: "old response lacks retention", status: 200, body: `{"protocol":"terminal-input-v1"}`, unsupported: true},
		{name: "retention disabled", status: 200, body: `{"retained":false}`, unsupported: true},
		{name: "authentication rejected", status: 401, body: `{"code":"unauthorized"}`},
		{name: "upstream failed", status: 502, body: `{"code":"remote_error"}`},
		{name: "invalid response", status: 200, body: `{"retained":"yes"}`},
		{name: "transport failed", err: errors.New("response unavailable")},
		{name: "cancelled", err: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &capabilityAgentStub{response: AgentResponse{StatusCode: tc.status, Body: []byte(tc.body)}, err: tc.err}
			_, err := (clusterTerminalSource{agent: agent}).OpenRetained(context.Background(), "owner", 24, 80)
			if err == nil || errors.Is(err, cluster.ErrTerminalRetentionUnavailable) != tc.unsupported {
				t.Fatalf("capability rejection = %v", err)
			}
			if agent.opened != 0 || agent.closed != 0 || len(agent.inputs) != 0 {
				t.Fatal("capability probe created, closed or wrote to a PTY")
			}
		})
	}
}

func TestRetainedAgentPreflightKeepsNewAgentInputAndRetentionCompatible(t *testing.T) {
	agent := &capabilityAgentStub{response: AgentResponse{StatusCode: http.StatusOK, Body: []byte(`{"protocol":"terminal-input-v1","retained":true}`)}}
	source := clusterTerminalSource{agent: agent}
	if !source.SupportsSequencedInput(context.Background()) {
		t.Fatal("retention advertisement broke input protocol decoding")
	}
	opened, err := source.OpenRetained(context.Background(), "owner", 24, 80)
	if err != nil || !opened.Retained || agent.opened != 1 {
		t.Fatalf("retained open = %#v, %v, opens=%d", opened, err, agent.opened)
	}
}
