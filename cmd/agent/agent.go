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
	EndpointURL string // 反向代理端
	//httpHandler http.Handler
}

type Option struct {
	AgentName   string
	EndpointURL string // 反向代理端
	GatewayHost string // websocket 服务端
}

func NewAgent(opt *Option) *Agent {
	a := &Agent{
		AgentName:   opt.AgentName,
		GatewayHost: opt.GatewayHost,
		EndpointURL: opt.EndpointURL,
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
	typ, rd, err := conn.NextReader()
	if err != nil {
		return err
	}
	if typ != websocket.BinaryMessage {

	}
	// 拿到request
	var (
		req *http.Request
	)
	{
		req, err = http.ReadRequest(bufio.NewReader(rd))
		if err != nil {
			return err
		}
		// 这里是硬编码
		req.URL.Path = a.trimPath(req.URL.Path)

		logrus.Debugf("get reqest. requestID:%s, path:%s", req.Header.Get(utils.HttpRequestIdHeader), req.URL.Path)
		//token := req.Header.Get("Token")
	}

	var (
		rw http.ResponseWriter
		buf = &bytes.Buffer{}
	)
	{

		rw = NewResponseWriter(buf)
		rw.Header().Set(utils.HttpRequestIdHeader, req.Header.Get(utils.HttpRequestIdHeader))
	}

	if a.EndpointURL != "" {
		u, err := url.Parse(a.EndpointURL)
		if err != nil {
			panic(err)
		}
		httpHandler := ReverseProxyHandler(u.Scheme, u.Host)
		httpHandler.ServeHTTP(rw, req)
	}

	//a.httpHandler = new(TestHandler)

	return conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
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

func (a *Agent) trimPath(path string) string {
	return strings.TrimPrefix(path, "/proxies/"+a.AgentName)
}
