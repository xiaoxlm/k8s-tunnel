package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	HttpRequestIdHeader = "X-Request-ID"

	// Time allowed to write a message to the peer.
	WriteWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	PongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod = (PongWait * 9) / 10

	// Time to wait before force close on connection.
	CloseGracePeriod = 10 * time.Second
)

func IsBrokenPipe(err error) bool {
	//if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
	//}

	return strings.Contains(err.Error(), "broken pipe")
}

func BuildResponse(content string) (*http.Response, error) {
	buf := bytes.Buffer{}
	buf.WriteString("HTTP/1.1 200 OK\r\n")
	buf.WriteString("Content-Type: application/json; charset=utf-8\r\n")
	buf.WriteString(fmt.Sprintf("\r\n%s\r\n", content))

	strReader := strings.NewReader(buf.String())

	return http.ReadResponse(bufio.NewReader(strReader), nil)
}
