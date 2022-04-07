package main

import (
	"bufio"
	"bytes"
	"github.com/gorilla/websocket"
	"net/http"
)

type TunnelRequestTransit struct {
	requestID string
	request   *http.Request
	RESP      chan *http.Response
}

func NewTunnelRequestTransit(requestID string, req *http.Request) *TunnelRequestTransit {
	return &TunnelRequestTransit{
		requestID: requestID,
		request:   req,
		RESP: make(chan *http.Response),
	}
}

func (rt *TunnelRequestTransit) Transit(conn *websocket.Conn) error {
	w, err := conn.NextWriter(websocket.BinaryMessage)

	if err != nil {
		return err
	}

	if err = rt.request.Write(w); err != nil {
		return err
	}

	return w.Close()
}

func (rt *TunnelRequestTransit) Response(conn *websocket.Conn) error {
	defer func() {
		close(rt.RESP)
	}()

	_, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(message)), rt.request)
	if err != nil {
		return err
	}

	rt.RESP <- resp
	return nil
}
