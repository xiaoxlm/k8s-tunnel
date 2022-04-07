package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"io"
	"k8s-tunnel/pkg/utils"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type Gateway struct {
	tunnelMap sync.Map // agentName:Tunnel
}

func NewGateway() *Gateway {
	return &Gateway{
		tunnelMap: sync.Map{},
	}
}

func (gw *Gateway) Serve(ctx context.Context) error {
	port := 9991
	server := &http.Server{}
	server.Addr = fmt.Sprintf(":%d", port)
	server.Handler = gw.NewRouter()

	go func() {
		fmt.Printf("listen on %d, (%s, %s)\n", port, runtime.GOOS, runtime.GOARCH)
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				fmt.Printf("server closed\n")
			} else {
				log.Fatal(err)
			}
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	<-stopCh

	timeout := 3 * time.Second
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	return server.Shutdown(ctx)
}

func (gw *Gateway) NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/agents/{agentName}/register", gw.registerHandler)
	r.PathPrefix("/proxies/{agentName}").HandlerFunc(gw.requestHandler)
	r.HandleFunc("/agents/{agentName}/response", gw.responseHandler)

	return r
}

func (gw *Gateway) registerHandler(writer http.ResponseWriter, request *http.Request) {
	if err := gw.authenticate(request); err != nil {
		RESP(writer, NewStatusErr(http.StatusUnauthorized, err))
		return
	}

	agentName := mux.Vars(request)["agentName"]
	if _, ok := gw.getTunnel(request); ok {
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: time.Second * 30,
		CheckOrigin: func(r *http.Request) bool {
			// 记得检查origin, 现在就统一返回
			return true
		},
	}

	conn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		panic(err)
	}

	tunnel := gw.initTunnel(agentName, conn)

	gw.tunnelMap.Store(agentName, tunnel)

	fmt.Printf("%s registerd\n", agentName)
}

func (gw *Gateway) responseHandler(writer http.ResponseWriter, request *http.Request) {
	if err := gw.authenticate(request); err != nil {
		RESP(writer, NewStatusErr(http.StatusUnauthorized, err))
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: time.Second * 30,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	requestID := request.Header.Get(utils.HttpRequestIdHeader)

	onceConn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		panic(err)
	}
	defer onceConn.Close()

	logrus.Debugf("once conn comming, requestID:%s", requestID)

	tunnel, ok := gw.getTunnel(request)
	if !ok {
		RESP(writer, NewStatusErr(http.StatusInternalServerError, fmt.Errorf("cant't get tunnel")))
		return
	}

	rt, err := tunnel.GetRequestTransit(requestID)
	if err != nil {
		RESP(writer, NewStatusErr(http.StatusConflict, err))
		return
	}
	logrus.Debugf("loading rt, requestID:%s", requestID)

	if err = rt.Transit(onceConn); err != nil {
		RESP(writer, NewStatusErr(http.StatusInternalServerError, err))
		return
	}

	if err = rt.Response(onceConn); err != nil {
		RESP(writer, NewStatusErr(http.StatusInternalServerError, err))
		return
	}

	tunnel.DeleteRequestTransit(requestID)
}

func (gw *Gateway) requestHandler(writer http.ResponseWriter, request *http.Request) {
	if err := gw.authenticate(request); err != nil {
		RESP(writer, NewStatusErr(http.StatusUnauthorized, err))
		return
	}

	tunnel, ok := gw.getTunnel(request)
	if !ok {
		RESP(writer, NewStatusErr(http.StatusInternalServerError, fmt.Errorf("cant't get tunnel")))
		return
	}

	resp, err := tunnel.HandleRequest(request)
	defer resp.Body.Close()
	if err != nil {
		RESP(writer, NewStatusErr(http.StatusGone, err))
		if utils.IsBrokenPipe(err) {
			tunnel.Close()
		}
		return
	}

	gw.response(resp, writer)
}

func (gw *Gateway) response(resp *http.Response, rw http.ResponseWriter) {
	for k, vv := range resp.Header {
		rw.Header()[k] = vv
	}

	rw.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(rw, resp.Body); err != nil {
		RESP(rw, NewStatusErr(http.StatusInternalServerError, err))
	}
}

func (gw *Gateway) authenticate(req *http.Request) error {
	return nil
}

func (gw *Gateway) initTunnel(agentName string, conn *websocket.Conn) *Tunnel {
	tunnel := NewTunnel(agentName, conn, gw)

	{ // handler
		tunnel.PongHandler()
		tunnel.CloseClientHandler()
	}

	go tunnel.SendPing()
	go tunnel.Recv()

	return tunnel
}

func (gw *Gateway) getTunnel(request *http.Request) (*Tunnel, bool) {
	agentName := mux.Vars(request)["agentName"]
	v, ok := gw.tunnelMap.Load(agentName)
	if !ok {
		return nil, false
	}

	return v.(*Tunnel), true
}

// error
func NewStatusErr(code int, err error) *StatusErr {
	if e, ok := err.(*StatusErr); ok {
		return e
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &StatusErr{Code: code, Msg: msg, err: err}
}

type StatusErr struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`

	err error
}

func (se *StatusErr) Error() string {
	return fmt.Sprintf("[%d] %+v", se.Code, se.err)
}

func RESP(rw http.ResponseWriter, err *StatusErr) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(err.Code)
	_ = json.NewEncoder(rw).Encode(err)
}

func (gw *Gateway) testHandler(writer http.ResponseWriter, request *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: time.Second * 30,
		CheckOrigin: func(r *http.Request) bool {
			// 记得检查origin, 现在就统一返回
			return true
		},
	}

	conn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		panic(err)
	}

	tunnel := gw.initTunnel("test", conn)

	go tunnel.WriteTest()
}

func (gw *Gateway) readTestHandler(writer http.ResponseWriter, request *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: time.Second * 30,
		CheckOrigin: func(r *http.Request) bool {
			// 记得检查origin, 现在就统一返回
			return true
		},
	}

	conn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		panic(err)
	}

	tunnel := gw.initTunnel("test", conn)

	tunnel.ReadTest()
}