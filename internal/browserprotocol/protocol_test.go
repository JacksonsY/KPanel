package browserprotocol

import "testing"

func TestCommandBoundary(t *testing.T) {
	for _, raw := range []string{`{"type":"Runtime.evaluate","text":"alert(1)"}`, `{"type":"navigate","url":"file:///etc/passwd"}`, `{"type":"new","url":"https://user:secret@example.com"}`, `{"type":"resize","width":9000,"height":720}`, `{"type":"mouse","action":"run"}`, `{"type":"text","text":"x","session":"other"}`, `{"type":"end"}{"type":"new"}`} {
		if _, err := DecodeCommand([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid command: %s", raw)
		}
	}
	for _, raw := range []string{`{"type":"new","url":"https://example.com"}`, `{"type":"text","text":"中文输入"}`, `{"type":"resize","width":640,"height":480}`, `{"type":"visible","visible":false}`, `{"type":"end"}`} {
		if _, err := DecodeCommand([]byte(raw)); err != nil {
			t.Fatal(raw, err)
		}
	}
}
