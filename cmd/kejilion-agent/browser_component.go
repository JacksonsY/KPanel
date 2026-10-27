package main

import (
	"context"
	"errors"
	"flag"
	"github.com/kejilion/kejilion-panel/internal/browsercomponent"
	"os"
	"os/signal"
	"syscall"
)

func runBrowserComponentJob(arguments []string) error {
	f := flag.NewFlagSet("browser-component-run", flag.ContinueOnError)
	dir := f.String("state-dir", "", "private component state directory")
	runtimeDir := f.String("runtime-dir", "", "browser runtime credential directory")
	socket := f.String("docker-socket", "", "Docker socket")
	id := f.String("job", "", "task identity")
	if err := f.Parse(arguments); err != nil {
		return err
	}
	if f.NArg() != 0 || *dir == "" || *runtimeDir == "" || *socket == "" || *id == "" {
		return errors.New("invalid browser component worker arguments")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return browsercomponent.New(browsercomponent.Config{StateDir: *dir, RuntimeDir: *runtimeDir, DockerSocket: *socket}).RunJob(ctx, *id)
}
