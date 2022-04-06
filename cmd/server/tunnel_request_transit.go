package main

import "net/http"

type TunnelRequestTransit struct {
	requestID string
	request   *http.Request
	tunnel    *Tunnel
}

func NewTunnelRequestTransit(requestID string, req *http.Request, tunnel *Tunnel) *TunnelRequestTransit {
	return &TunnelRequestTransit{
		requestID: requestID,
		request:   req,
		tunnel:    tunnel,
	}
}
