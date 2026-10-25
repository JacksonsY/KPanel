package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/virtualmachine"
)

func runVMJob(arguments []string) error {
	flags := flag.NewFlagSet("vm-run", flag.ContinueOnError)
	dir := flags.String("state-dir", "", "private VM task directory")
	id := flags.String("job", "", "durable task identity")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *dir == "" || !contract.ValidVMID(*id) {
		return errors.New("vm-run requires a state directory and task UUID")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return virtualmachine.New(virtualmachine.Config{StateDir: *dir, Enabled: true}).RunJob(ctx, *id)
}
