// kpanel-browser runs separately from Panel and Agent as an unprivileged user.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/kejilion/kejilion-panel/internal/webbrowser"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	listen := flag.String("listen", "127.0.0.1:8092", "private listen address")
	browser := flag.String("browser", "", "absolute path of an installed Chromium executable")
	tokenFile := flag.String("token-file", "", "shared service token file (at least 32 bytes)")
	private := flag.String("private-cidrs", "", "explicitly permitted private application networks, comma separated")
	flag.Parse()
	if !filepath.IsAbs(*browser) || !filepath.IsAbs(*tokenFile) {
		return fmt.Errorf("browser and token-file must be absolute paths")
	}
	file, err := os.Open(*tokenFile)
	if err != nil {
		return fmt.Errorf("browser token file unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 256 || (runtime.GOOS != "windows" && info.Mode().Perm()&0007 != 0) {
		return fmt.Errorf("browser token file must be private and at most 256 bytes")
	}
	data := make([]byte, 257)
	n, err := file.Read(data)
	if err != nil {
		return fmt.Errorf("browser token file unavailable")
	}
	var cidrs []netip.Prefix
	if *private != "" {
		for _, value := range strings.Split(*private, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("invalid private-cidrs")
			}
			cidrs = append(cidrs, prefix)
		}
	}
	service, err := webbrowser.NewService(*browser, strings.TrimSpace(string(data[:n])), cidrs)
	if err != nil {
		return err
	}
	defer service.Close()
	server := &http.Server{Addr: *listen, Handler: service, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() { <-ctx.Done(); service.Close(); _ = server.Close() }()
	err = server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
