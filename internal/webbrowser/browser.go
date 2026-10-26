package webbrowser

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type browserTab struct {
	Tab
	ctx       context.Context
	cancel    context.CancelFunc
	streaming bool
}
type browserSession struct {
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	allocatorCancel context.CancelFunc
	root            context.Context
	rootCancel      context.CancelFunc
	proxy           *Egress
	proxyServer     *http.Server
	tabs            []*browserTab
	active          string
	width, height   int64
	visible         bool
	nextID          int
	dirty           chan struct{}
	frames          chan Event
	notices         chan Event
}

func call[P, R any](ctx context.Context, command cdp.Command[P, R], params P) (R, error) {
	return chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) (R, error) { return cdp.Call(ctx, t, command, params) })
}
func send[P any](ctx context.Context, command cdp.Command[P, cdp.Empty], params P) error {
	_, err := call(ctx, command, params)
	if err != nil {
		return fmt.Errorf("%s: %w", command.Method, err)
	}
	return err
}
func browserCall[P, R any](ctx context.Context, command cdp.Command[P, R], params P) (R, error) {
	return cdp.Call(ctx, chromedp.FromContext(ctx).Browser, command, params)
}
func newBrowserSession(parent context.Context) *browserSession {
	ctx, cancel := context.WithCancel(parent)
	return &browserSession{ctx: ctx, cancel: cancel, width: 1280, height: 720, visible: true, dirty: make(chan struct{}, 1), frames: make(chan Event, 1), notices: make(chan Event, 8)}
}
func (s *browserSession) signal() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}
func (s *browserSession) notice(e Event) {
	select {
	case s.notices <- e:
	default:
	}
}
func (s *browserSession) Close() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopBrowser()
}

func (s *browserSession) stopBrowser() {
	if s.rootCancel != nil {
		s.rootCancel()
	}
	if s.allocatorCancel != nil {
		s.allocatorCancel()
	}
	if s.proxyServer != nil {
		_ = s.proxyServer.Close()
	}
	if s.proxy != nil {
		s.proxy.Close()
	}
	s.root = nil
	s.rootCancel = nil
	s.allocatorCancel = nil
	s.tabs = nil
	s.active = ""
}
func (s *browserSession) start(executable string, private []netip.Prefix) error {
	if s.root != nil {
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.New("browser_start_failed")
	}
	s.proxy = NewEgress(private)
	s.proxyServer = &http.Server{Handler: s.proxy, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 30 * time.Minute, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 << 10}
	go func() { _ = s.proxyServer.Serve(listener) }()
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	options = append(options, chromedp.ExecPath(executable), chromedp.ProxyServer("http://"+listener.Addr().String()), chromedp.Flag("no-sandbox", false), chromedp.Flag("proxy-bypass-list", "<-loopback>"), chromedp.Flag("disable-quic", true), chromedp.Flag("force-webrtc-ip-handling-policy", "disable_non_proxied_udp"), chromedp.Flag("host-resolver-rules", "MAP * ~NOTFOUND, EXCLUDE 127.0.0.1"), chromedp.Flag("disable-features", "Translate,BlinkGenPropertyTrees,MediaRouter"), chromedp.Flag("disable-popup-blocking", false), chromedp.Flag("disk-cache-size", "33554432"), chromedp.Flag("media-cache-size", "16777216"))
	allocator, stopAllocator := chromedp.NewExecAllocator(s.ctx, options...)
	s.allocatorCancel = stopAllocator
	root, stopRoot := chromedp.NewContext(allocator)
	// The initial context owns the process; a per-command timeout would kill it.
	if err := chromedp.Do(root); err != nil {
		stopRoot()
		stopAllocator()
		_ = s.proxyServer.Close()
		s.proxy.Close()
		return errors.New("browser_start_failed")
	}
	s.root, s.rootCancel = root, stopRoot
	ctx, cancel := context.WithTimeout(root, 5*time.Second)
	defer cancel()
	enabled := true
	if _, err := browserCall(ctx, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{Behavior: browser.SetDownloadBehaviorBehaviorDeny, EventsEnabled: &enabled}); err != nil {
		s.stopBrowser()
		return errors.New("browser_start_failed")
	}
	s.listenBrowser()
	return nil
}

func (s *browserSession) listenBrowser() {
	root := s.root
	created := chromedp.BrowserEvents(root, target.TargetCreated)
	go func() {
		for event, err := range created {
			if err != nil {
				return
			}
			info := event.TargetInfo
			if info.Type != "page" || info.OpenerID == "" {
				continue
			}
			s.mu.Lock()
			if len(s.tabs) >= MaxTabs || s.root != root {
				ctx, cancel := context.WithTimeout(root, 2*time.Second)
				_, _ = browserCall(ctx, target.CloseTarget, target.CloseTargetParams{TargetID: info.TargetID})
				cancel()
				s.mu.Unlock()
				s.notice(Event{Type: "error", Code: "tab_limit"})
				continue
			}
			s.nextID++
			ctx, cancel := chromedp.NewContext(root, chromedp.WithTargetID(info.TargetID))
			tab := &browserTab{Tab: Tab{ID: fmt.Sprintf("tab-%d", s.nextID), URL: info.URL, Title: info.Title}, ctx: ctx, cancel: cancel}
			if chromedp.Do(ctx) != nil {
				cancel()
				s.mu.Unlock()
				continue
			}
			s.listen(tab)
			s.tabs = append(s.tabs, tab)
			s.active = tab.ID
			_ = s.stream()
			s.mu.Unlock()
			s.signal()
		}
	}()
	downloads := chromedp.BrowserEvents(root, browser.DownloadWillBegin)
	go func() {
		for _, err := range downloads {
			if err != nil {
				return
			}
			s.notice(Event{Type: "error", Code: "download_unavailable"})
		}
	}()
}
func (s *browserSession) selected(id string) *browserTab {
	if id == "" {
		id = s.active
	}
	for _, tab := range s.tabs {
		if tab.ID == id {
			return tab
		}
	}
	return nil
}
func (s *browserSession) listen(tab *browserTab) {
	frames := chromedp.Events(tab.ctx, page.ScreencastFrame)
	go func() {
		var next time.Time
		for event, err := range frames {
			if err != nil {
				return
			}
			if !time.Now().Before(next) && len(event.Data) < MaxEventBytes/2 {
				next = time.Now().Add(100 * time.Millisecond)
				frame := Event{Type: "frame", Tab: tab.ID, Image: base64.StdEncoding.EncodeToString(event.Data), Width: int64(event.Metadata.DeviceWidth), Height: int64(event.Metadata.DeviceHeight)}
				select {
				case s.frames <- frame:
				default:
				}
			}
			ctx, cancel := context.WithTimeout(tab.ctx, 3*time.Second)
			// ACK pacing bounds Chromium capture/encoding, not just client delivery.
			select {
			case <-ctx.Done():
				cancel()
				return
			case <-time.After(100 * time.Millisecond):
			}
			err = send(ctx, page.ScreencastFrameAck, page.ScreencastFrameAckParams{SessionID: event.SessionID})
			cancel()
			if err != nil {
				return
			}
		}
	}()
	dialogs := chromedp.Events(tab.ctx, page.JavascriptDialogOpening)
	go func() {
		for event, err := range dialogs {
			if err != nil {
				return
			}
			text := event.Message
			if len(text) > 2048 {
				text = text[:2048]
			}
			s.notice(Event{Type: "dialog", Tab: tab.ID, Code: string(event.Type), Text: text})
		}
	}()
	files := chromedp.Events(tab.ctx, page.FileChooserOpened)
	go func() {
		for _, err := range files {
			if err != nil {
				return
			}
			s.notice(Event{Type: "error", Tab: tab.ID, Code: "upload_unavailable"})
		}
	}()
}
func (s *browserSession) stream() error {
	for _, tab := range s.tabs {
		want := s.visible && tab.ID == s.active
		if tab.streaming == want {
			continue
		}
		ctx, cancel := context.WithTimeout(tab.ctx, 5*time.Second)
		var err error
		if want {
			quality := int64(70)
			err = send(ctx, page.BringToFront, cdp.Empty{})
			if err == nil {
				err = send(ctx, page.StartScreencast, page.StartScreencastParams{Format: page.StartScreencastFormatJpeg, Quality: &quality, MaxWidth: s.width, MaxHeight: s.height, MaxFramesInFlight: 2})
			}
		} else {
			err = send(ctx, page.StopScreencast, cdp.Empty{})
		}
		cancel()
		if err != nil {
			return errors.New("browser_disconnected")
		}
		tab.streaming = want
	}
	return s.ctx.Err()
}
func (s *browserSession) Execute(c Command, executable string, private []netip.Prefix) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.signal()
	if s.root != nil && s.root.Err() != nil {
		s.stopBrowser()
	}
	if c.Type == "visible" {
		s.visible = c.Visible
		if s.root == nil {
			return nil
		}
		return s.stream()
	}
	if c.Type == "resize" {
		s.width, s.height = c.Width, c.Height
		if s.root == nil {
			return nil
		}
	}
	if c.Type == "new" {
		if len(s.tabs) >= MaxTabs {
			return errors.New("tab_limit")
		}
		if err := s.start(executable, private); err != nil {
			return err
		}
		s.nextID++
		ctx, cancel := chromedp.NewContext(s.root)
		tab := &browserTab{Tab: Tab{ID: fmt.Sprintf("tab-%d", s.nextID), URL: "about:blank"}, ctx: ctx, cancel: cancel}
		if err := chromedp.Do(ctx); err != nil {
			cancel()
			return errors.New("browser_start_failed")
		}
		s.listen(tab)
		run, stop := context.WithTimeout(ctx, 5*time.Second)
		err := send(run, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: s.width, Height: s.height, DeviceScaleFactor: 1})
		if err == nil {
			err = send(run, page.SetInterceptFileChooserDialog, page.SetInterceptFileChooserDialogParams{Enabled: true})
		}
		stop()
		if err != nil {
			cancel()
			return errors.New("browser_start_failed")
		}
		s.tabs = append(s.tabs, tab)
		s.active = tab.ID
		if c.URL != "" {
			c.Type = "navigate"
			c.Tab = tab.ID
		} else {
			return s.stream()
		}
	}
	tab := s.selected(c.Tab)
	if tab == nil {
		return errors.New("tab_unavailable")
	}
	ctx, stop := context.WithTimeout(tab.ctx, 10*time.Second)
	defer stop()
	var err error
	switch c.Type {
	case "navigate":
		if _, err := WebURL(c.URL); err != nil {
			return err
		}
		tab.Loading = true
		err = chromedp.Do(ctx, chromedp.Navigate(c.URL))
	case "select":
		s.active = tab.ID
	case "close":
		tab.cancel()
		for i, t := range s.tabs {
			if t == tab {
				s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
				break
			}
		}
		if s.active == tab.ID {
			s.active = ""
			if len(s.tabs) > 0 {
				s.active = s.tabs[len(s.tabs)-1].ID
			}
		}
		if len(s.tabs) == 0 {
			s.stopBrowser()
			return nil
		}
	case "back", "forward":
		history, e := call(ctx, page.GetNavigationHistory, cdp.Empty{})
		err = e
		if e == nil {
			index := history.CurrentIndex
			if c.Type == "back" {
				index--
			} else {
				index++
			}
			if index >= 0 && index < int64(len(history.Entries)) {
				err = send(ctx, page.NavigateToHistoryEntry, page.NavigateToHistoryEntryParams{EntryID: history.Entries[index].ID})
			}
		}
	case "reload":
		err = send(ctx, page.Reload, page.ReloadParams{})
	case "stop":
		err = send(ctx, page.StopLoading, cdp.Empty{})
	case "resize":
		err = send(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: c.Width, Height: c.Height, DeviceScaleFactor: 1})
		if tab.streaming {
			_ = send(ctx, page.StopScreencast, cdp.Empty{})
			tab.streaming = false
		}
	case "text":
		err = send(ctx, input.InsertText, input.InsertTextParams{Text: c.Text})
	case "key":
		vk := map[string]int64{"Enter": 13, "Tab": 9, "Backspace": 8, "Delete": 46, "Escape": 27, "ArrowLeft": 37, "ArrowUp": 38, "ArrowRight": 39, "ArrowDown": 40, "Home": 36, "End": 35, "PageUp": 33, "PageDown": 34}[c.Key]
		if len(c.Key) == 1 && c.Key[0] >= 'a' && c.Key[0] <= 'z' {
			vk = int64(c.Key[0] - 'a' + 'A')
		}
		err = send(ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: input.DispatchKeyEventType(c.Action), Key: c.Key, Code: c.Code, Modifiers: c.Modifiers, WindowsVirtualKeyCode: vk})
	case "mouse":
		if c.X < 0 || c.Y < 0 || c.X > float64(s.width) || c.Y > float64(s.height) {
			return errors.New("invalid_coordinates")
		}
		p := input.DispatchMouseEventParams{Type: input.DispatchMouseEventType(c.Action), X: c.X, Y: c.Y, Modifiers: c.Modifiers, DeltaX: c.DeltaX, DeltaY: c.DeltaY, Button: input.MouseButton(c.Button)}
		if c.Action == "mousePressed" || c.Action == "mouseReleased" {
			p.ClickCount = 1
		}
		err = send(ctx, input.DispatchMouseEvent, p)
	case "dialog":
		err = send(ctx, page.HandleJavaScriptDialog, page.HandleJavaScriptDialogParams{Accept: c.Accept, PromptText: c.Text})
	default:
		return errors.New("invalid_command")
	}
	if err != nil {
		return errors.New("browser_action_failed")
	}
	return s.stream()
}
func (s *browserSession) Snapshot() Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	event := Event{Type: "state", Active: s.active, Tabs: make([]Tab, 0, len(s.tabs))}
	for _, tab := range s.tabs {
		ctx, cancel := context.WithTimeout(tab.ctx, 2*time.Second)
		history, err := call(ctx, page.GetNavigationHistory, cdp.Empty{})
		if err == nil && history.CurrentIndex >= 0 && history.CurrentIndex < int64(len(history.Entries)) {
			index := history.CurrentIndex
			entry := history.Entries[index]
			tab.URL = entry.URL
			tab.Title = entry.Title
			if len(tab.Title) > 160 {
				tab.Title = tab.Title[:160]
			}
			if len(tab.URL) > 4096 {
				tab.URL = tab.URL[:4096]
			}
			tab.Back = index > 0
			tab.Forward = index+1 < int64(len(history.Entries))
		}
		if title, e := chromedp.Run(ctx, chromedp.Title()); e == nil && len(title) <= 160 {
			tab.Title = title
		}
		if loading, e := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.readyState !== 'complete'`)); e == nil {
			tab.Loading = loading
		}
		cancel()
		event.Tabs = append(event.Tabs, tab.Tab)
	}
	return event
}
