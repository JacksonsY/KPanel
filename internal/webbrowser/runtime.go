package webbrowser

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/chromedp/chromedp"
)

// CheckRuntime tests the actual launcher and browser sandbox, before healthz
// can advertise readiness. It neither visits a website nor exposes CDP.
func CheckRuntime(executable string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	s := newBrowserSession(ctx)
	defer s.Close()
	if err := s.start(executable, nil); err != nil {
		return errors.New("browser_runtime_unsupported: Chromium could not start with its sandbox")
	}
	if _, err := chromedp.Run(s.root, chromedp.Navigate("chrome://sandbox/")); err != nil {
		return errors.New("browser_runtime_unsupported: sandbox status unavailable")
	}
	status, err := chromedp.Run(s.root, chromedp.Evaluate[string](`document.body.innerText`))
	if err != nil || !sandboxEnabled(status) {
		return errors.New("browser_runtime_unsupported: Chromium namespace or seccomp sandbox unavailable")
	}
	return nil
}

func sandboxEnabled(status string) bool {
	return regexp.MustCompile(`PID namespaces\s+Yes`).MatchString(status) &&
		regexp.MustCompile(`Seccomp-BPF sandbox\s+Yes`).MatchString(status)
}
