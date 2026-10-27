package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	token, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	ws, _, err := websocket.Dial(ctx, "ws://"+os.Args[1]+":8092/v1/sessions/"+strings.Repeat("a", 64), &websocket.DialOptions{HTTPClient: &http.Client{Transport: &http.Transport{Proxy: nil}}, HTTPHeader: http.Header{"Authorization": {"Bearer " + string(token)}}})
	if err != nil {
		panic(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(4 << 20)
	_, _, err = ws.Read(ctx)
	if err != nil {
		panic(err)
	}
	_ = ws.Write(ctx, websocket.MessageText, []byte(`{"type":"visible","visible":true}`))
	_ = ws.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","width":1280,"height":720}`))
	err = ws.Write(ctx, websocket.MessageText, []byte(`{"type":"new","url":"https://example.com"}`))
	if err != nil {
		panic(err)
	}
	frame, title := false, false
	for !(frame && title) {
		_, data, err := ws.Read(ctx)
		if err != nil {
			panic(err)
		}
		var event struct {
			Type  string
			Image string
			Code  string
			Tabs  []struct{ Title string }
		}
		if json.Unmarshal(data, &event) != nil {
			panic("invalid browser event")
		}
		if event.Type == "error" {
			panic("browser error: " + event.Code)
		}
		if event.Type == "frame" && event.Image != "" {
			if !frame {
				fmt.Println("probe frame=received")
			}
			frame = true
		}
		for _, tab := range event.Tabs {
			if tab.Title != "" {
				fmt.Println("probe title=" + tab.Title)
			}
			if tab.Title == "Example Domain" {
				title = true
			}
		}
	}
	_ = ws.Write(ctx, websocket.MessageText, []byte(`{"type":"end"}`))
	fmt.Println("live-browser=pass title=Example_Domain frame=received authenticated_websocket=pass")
}
