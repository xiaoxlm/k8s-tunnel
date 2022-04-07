package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"k8s-tunnel/pkg/log"
	"k8s-tunnel/pkg/utils"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func init() {
	log.LogInit("agent", logrus.DebugLevel)
}

// server 断开连接， client要定时去重新建立连接

type Agent struct {
	AgentName   string
	GatewayHost string
	conn        *websocket.Conn
	done        chan struct{}
}

type Option struct {
	AgentName   string
	GatewayHost string // websocket 服务端
}

func NewAgent(opt *Option) *Agent {
	a := &Agent{
		AgentName:   opt.AgentName,
		GatewayHost: opt.GatewayHost,
		done:        make(chan struct{}),
	}

	return a
}

func (a *Agent) Serve() {
	ctx := context.Background()

	{
		a.connect()
		logrus.Debugf("dial %s success", a.GatewayHost)
		a.PingHandler()
		go a.SendPing(ctx)
	}

	go func() {
		for {
			err := a.HandleRequest()
			if err != nil {
				logrus.Errorf("handler request error. err:%v", err)
				time.Sleep(utils.PingPeriod)
			}
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	<-stopCh

	a.Close(context.Background())
	logrus.Debugf("agent exit.")
	os.Exit(-1)
}

func (a *Agent) HandleRequest() error {
	conn := a.GetConn()
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	switch messageType {
	case websocket.BinaryMessage:
		return nil
	case websocket.CloseMessage:
		return nil
	case websocket.PingMessage:
		return nil
	case websocket.PongMessage:
		return nil
	}

	go func(requestID string) {
		logrus.Debugf("agent get requestID: %s", requestID)
		ctx := context.Background()
		if err = a.response(ctx, requestID); err != nil {
			logrus.Errorf("response error. requestID:%s, err:%v", requestID, err)
		}
	}(string(message))

	return nil
}

func (a *Agent) GetConn() *websocket.Conn {
	return a.conn
}

func (a *Agent) Dial(ctx context.Context, path string, headers http.Header) error {
	u := url.URL{Scheme: "ws", Host: a.GatewayHost, Path: path}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return err
	}
	a.conn = conn

	return nil
}

func (a *Agent) SendPing(ctx context.Context) {
	ticker := time.NewTicker(utils.PingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := a.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(utils.PingPeriod + time.Second)); err != nil {
				logrus.Errorf("ping error: %v", err)
				// reset
				a.Reset(ctx)
			}
			// ping 通，即可设置write deadline
			_ = a.conn.SetWriteDeadline(time.Now().Add(31 * time.Second))

		case <-a.done:
			return
		}
	}
}

func (a *Agent) Reset(ctx context.Context) {
	a.Close(ctx)
	time.Sleep(1 * time.Second)
	a.done = make(chan struct{})
	a.connect()
	logrus.Debug("reset success")
}

func (a *Agent) Close(ctx context.Context) {
	close(a.done)
	a.conn.Close()
}

// 处理ping消息
func (a *Agent) PingHandler() {
	a.conn.SetPingHandler(func(appData string) error {
		return a.conn.WriteMessage(websocket.PongMessage, nil)
	})
}

func (a *Agent) connect() {
	path := fmt.Sprintf("/agents/%s/register", a.AgentName)
	err := a.Dial(context.Background(), path, nil)
	if err != nil {
		logrus.Errorf("register invalid. err:%v", err)
		return
	}
}

func (a *Agent) response(ctx context.Context, requestID string) error {
	path := fmt.Sprintf("/agents/%s/response", a.AgentName)
	u := url.URL{Scheme: "ws", Host: a.GatewayHost, Path: path}

	header := http.Header{}
	header.Add(utils.HttpRequestIdHeader, requestID)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return err
	}
	defer func() {
		if err = conn.Close(); err != nil {
			logrus.Errorf("response conn close error. err:%v", err)
			return
		}
	}()
	logrus.Debugf("start response conn, requestID:%s", requestID)

	// get k8s request
	var (
		req *http.Request // k8s request
	)
	{
		req, err = a.parseK8sRequest(conn)
		if err != nil {
			return err
		}
	}

	// write
	var (
		rw http.ResponseWriter
		buf = &bytes.Buffer{}
	)
	{
		rw = NewResponseWriter(buf)
		rw.Header().Set(utils.HttpRequestIdHeader, requestID)
	}

	httpHandler, err := K8sReverseProxyHandler()
	if err != nil {
		return err
	}
	httpHandler.ServeHTTP(rw, req)
	err = conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())

	logrus.Debugf("agent write back k8s request, requestID:%s", requestID)
	return err
}

func (a *Agent) parseK8sRequest(onceConn *websocket.Conn) (*http.Request, error) {
	typ, message, err := onceConn.ReadMessage()
	if err != nil {
		logrus.Errorf("conn ReadMessage error. err:%v", err)
		return nil, err
	}
	if typ != websocket.BinaryMessage {
		return nil, fmt.Errorf("type is not binary")
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(message)))
	if err != nil {
		logrus.Errorf("ReadRequest error. err:%v", err)
		return nil, err
	}
	req.URL.Path = a.trimPath(req.URL.Path)
	logrus.Debugf("agent get reqest requestID:%s, path:%s", req.Header.Get(utils.HttpRequestIdHeader), req.URL.Path)
	return req, nil
}

func (a *Agent) trimPath(path string) string {
	return strings.TrimPrefix(path, "/proxies/"+a.AgentName)
}
