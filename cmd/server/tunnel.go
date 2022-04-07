package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"k8s-tunnel/pkg/utils"
	"net/http"
	"sync"
	"time"
)

type Tunnel struct {
	Name string
	conn      *websocket.Conn
	gateway   *Gateway
	done      chan struct{}
	requests  sync.Map // requestID: *TunnelRequestTransit
}

func NewTunnel(agentName string, conn *websocket.Conn, gateway *Gateway) *Tunnel {
	return &Tunnel{
		Name: agentName,
		conn:      conn,
		gateway:   gateway,
		done:      make(chan struct{}),
		requests:  sync.Map{},
	}
}

// 向客户端发送 ping
// WriteControl 可以针对每种数据类型，进行设置write deadline
func (t *Tunnel) SendPing() {
	ticker := time.NewTicker(utils.PingPeriod / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := t.conn.WriteControl(websocket.PingMessage, []byte("ping ping ping"), time.Now().Add(utils.PingPeriod+time.Second)); err != nil {
				logrus.Errorf("ping invalid: %v", err)
				t.Close()
				return
			}
			// ping 通，即可设置write deadline
			_ = t.conn.SetWriteDeadline(time.Now().Add(utils.WriteWait))

		case <-t.done:
			return
		}
	}
}

// 处理客户端返回 pong
func (t *Tunnel) PongHandler() {
	t.conn.SetPongHandler(func(appData string) error {
		// 由于业务的特殊性，不用设置读超时
		//return t.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})
}

// client close后的处理
// nead read
func (t *Tunnel) CloseClientHandler() {
	t.conn.SetCloseHandler(func(code int, str string) error {
		t.Close()
		//t.gateway.tunnelMap.Delete(t.agentName)
		//// 当关闭的时候，让协程退出
		//close(t.done)
		//fmt.Println("tunnel close")
		return nil
	})
}

// 关闭主动连接
func (t *Tunnel) Close() {
	t.gateway.tunnelMap.Delete(t.Name)
	// 当关闭的时候，让协程退出
	close(t.done)
	t.conn.Close()
	fmt.Printf("%s tunnel closed.\n", t.Name)
}

func (t *Tunnel) HandlerRequest(req *http.Request) (*http.Response, error) {
	w, err := t.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		fmt.Println("NextWriter error")
		return nil, err
	}

	{
		// requestID
		requestID := uuid.New().String()
		t.requests.Store(requestID, req)
		req.Header.Add(utils.HttpRequestIdHeader, requestID)
	}

	// 将 request 全量发送到客户端
	if err = req.Write(w); err != nil {
		fmt.Println("write error")
		return nil, err
	}

	if err = w.Close(); err != nil {
		return nil, err
	}

	// read
	_, reader, err := t.conn.NextReader()
	if err != nil {
		fmt.Println("NextReader error")
		return nil, err
	}

	b, _ := ioutil.ReadAll(reader)
	buf := bytes.NewReader(b)
	return http.ReadResponse(bufio.NewReader(buf), req)
	//requestID := resp.Header.Get(utils.HttpRequestIdHeader)
	//request, ok := t.requests.Load(requestID)
	//if !ok {
	//	return nil, fmt.Errorf("requestID:%s invalid", requestID)
	//}
	//resp.Request = request.(*http.Request)

	//return resp, nil
	//return t.RESPTest(reader)
}

/***test func***/
func (t *Tunnel) WriteTest() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			// for close handler
			_, r, err := t.conn.NextReader()
			if err != nil {
				return
			}

			b, _ := ioutil.ReadAll(r)
			fmt.Println(string(b))
		}

	}()

	for tk := range ticker.C {
		err := t.conn.WriteMessage(websocket.TextMessage, []byte("from server:"+tk.String()))
		if err != nil {
			fmt.Printf("server write message error. err:%v\n", err)
			if utils.IsBrokenPipe(err) {
				t.Close()
				return
			}
		}
	}
}

func (t *Tunnel) ReadTest() {

	for {
		// for close handler
		_, r, err := t.conn.NextReader()
		if err != nil {
			logrus.Errorf("reader error. err:%v", err)
			return
		}

		b, _ := ioutil.ReadAll(r)
		fmt.Println(string(b))
	}
}

func (t *Tunnel) ResponseTest(reader io.Reader) (*http.Response, error) {
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return utils.BuildResponse(string(b))
}
/***test func ***/
