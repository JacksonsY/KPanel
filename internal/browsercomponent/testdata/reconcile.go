package main

import (
	"context"
	"os"
	"time"

	"github.com/kejilion/kejilion-panel/internal/browsercomponent"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	browsercomponent.New(browsercomponent.Config{StateDir: os.Args[1], RuntimeDir: os.Args[2]}).RunNetworkReconciler(ctx)
}
