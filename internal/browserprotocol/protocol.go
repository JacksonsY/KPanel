// Package browserprotocol defines the optional remote browser's bounded wire format.
// Its wire protocol intentionally exposes a small set of actions, never CDP.
package browserprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/url"
	"strings"
)

const MaxTabs = 8
const MaxCommandBytes = 16 << 10
const MaxEventBytes = 2 << 20

type Command struct {
	Type      string  `json:"type"`
	Tab       string  `json:"tab,omitempty"`
	URL       string  `json:"url,omitempty"`
	Text      string  `json:"text,omitempty"`
	Key       string  `json:"key,omitempty"`
	Code      string  `json:"code,omitempty"`
	Action    string  `json:"action,omitempty"`
	Button    string  `json:"button,omitempty"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	DeltaX    float64 `json:"deltaX,omitempty"`
	DeltaY    float64 `json:"deltaY,omitempty"`
	Modifiers int64   `json:"modifiers,omitempty"`
	Width     int64   `json:"width,omitempty"`
	Height    int64   `json:"height,omitempty"`
	Visible   bool    `json:"visible,omitempty"`
	Accept    bool    `json:"accept,omitempty"`
}

type Tab struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Loading bool   `json:"loading"`
	Back    bool   `json:"back"`
	Forward bool   `json:"forward"`
}

type Event struct {
	Type   string `json:"type"`
	Tab    string `json:"tab,omitempty"`
	Active string `json:"active"`
	Tabs   []Tab  `json:"tabs"`
	Image  string `json:"image,omitempty"`
	Width  int64  `json:"width,omitempty"`
	Height int64  `json:"height,omitempty"`
	Code   string `json:"code,omitempty"`
	Text   string `json:"text,omitempty"`
}

func WebURL(raw string) (string, error) {
	if len(raw) == 0 || len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\x00") {
		return "", errors.New("invalid_url")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("invalid_url")
	}
	return u.String(), nil
}

func DecodeCommand(data []byte) (Command, error) {
	var c Command
	if len(data) > MaxCommandBytes {
		return c, errors.New("command_too_large")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, errors.New("invalid_command")
	}
	if d.Decode(new(any)) != io.EOF {
		return c, errors.New("invalid_command")
	}
	if len(c.Tab) > 64 || len(c.Text) > 8192 || len(c.Key) > 40 || len(c.Code) > 40 || c.Modifiers < 0 || c.Modifiers > 15 {
		return c, errors.New("invalid_command")
	}
	for _, n := range []float64{c.X, c.Y, c.DeltaX, c.DeltaY} {
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 10000 {
			return c, errors.New("invalid_command")
		}
	}
	switch c.Type {
	case "new", "navigate":
		if c.URL != "" {
			if _, err := WebURL(c.URL); err != nil {
				return c, err
			}
		} else if c.Type == "navigate" {
			return c, errors.New("invalid_url")
		}
	case "select", "close", "back", "forward", "reload", "stop", "visible", "dialog", "end":
	case "resize":
		if c.Width < 320 || c.Width > 1600 || c.Height < 240 || c.Height > 1000 {
			return c, errors.New("invalid_viewport")
		}
	case "text":
		if c.Text == "" {
			return c, errors.New("invalid_command")
		}
	case "key":
		if c.Action != "keyDown" && c.Action != "keyUp" {
			return c, errors.New("invalid_command")
		}
	case "mouse":
		if c.Action != "mouseMoved" && c.Action != "mousePressed" && c.Action != "mouseReleased" && c.Action != "mouseWheel" {
			return c, errors.New("invalid_command")
		}
		if c.Button != "" && c.Button != "none" && c.Button != "left" && c.Button != "middle" && c.Button != "right" {
			return c, errors.New("invalid_command")
		}
	default:
		return c, errors.New("invalid_command")
	}
	return c, nil
}
