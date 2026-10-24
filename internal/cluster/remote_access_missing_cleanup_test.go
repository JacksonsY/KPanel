package cluster

import (
	"context"
	"github.com/kejilion/kejilion-panel/internal/terminal"
	"testing"
)

func TestIndependentRepairMissingCleanupSessionIsConfirmed(t *testing.T) {
	for _, missing := range []error{terminal.ErrNotFound, terminal.ErrClosed} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		stream := &streamTerminal{ctx: ctx, cancel: cancel, buffer: terminal.NewBuffer(terminal.DefaultBufferBytes, 0), cleanupDial: func(context.Context, byte, any) (*fileStreamConn, terminalStreamReady, error) {
			return nil, terminalStreamReady{}, missing
		}}
		if err := stream.close(context.Background(), TerminalCloseRequest{SessionID: "gone"}); err != nil {
			t.Errorf("confirmed missing session should release retired handle: %v", err)
		}
		if !stream.closing {
			t.Error("confirmed missing session retained retired handle")
		}
	}
}
