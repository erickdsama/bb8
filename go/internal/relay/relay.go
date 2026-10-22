// Package relay carries the web pages' HTTP requests to the robot over a
// connection the robot opens itself. bb8-web (the page, off the robot) never
// dials the robot: bb8-body dials out to it with a device token, like the head
// dials the body, so the robot opens no ports and works behind any home router.
//
// One WebSocket carries many requests at once. Each request is one frame; the
// reply is a frame with the status and headers, then body chunks as the robot
// writes them (so SSE streams flow live), then an end frame. Closing a page
// sends a cancel frame and the robot stops that handler.
package relay

// Frame is every message on the link, as JSON.
type Frame struct {
	ID     uint64              `json:"id,omitempty"`
	Method string              `json:"method,omitempty"` // request
	Path   string              `json:"path,omitempty"`   // request: path and query
	Header map[string][]string `json:"header,omitempty"` // request or reply head
	Status int                 `json:"status,omitempty"` // reply head
	Body   []byte              `json:"body,omitempty"`   // request body or a reply chunk
	End    bool                `json:"end,omitempty"`    // reply finished
	Cancel bool                `json:"cancel,omitempty"` // the page went away
	Ping   bool                `json:"ping,omitempty"`   // keepalive, both ways
}

const (
	// MaxBody is the largest request body the page may send.
	MaxBody = 1 << 20
	// PingEvery keeps the link alive through NATs; a side silent for
	// 3×PingEvery is dead.
	PingEvery = 20e9 // ns
)

// hop-by-hop headers stay on their own side of the link.
var hop = map[string]bool{"Connection": true, "Keep-Alive": true, "Transfer-Encoding": true, "Upgrade": true,
	"Proxy-Connection": true, "Te": true, "Trailer": true, "Cookie": true, "Authorization": true, "Content-Length": true}

func copyHeader(h map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range h {
		if !hop[k] {
			out[k] = v
		}
	}
	return out
}
