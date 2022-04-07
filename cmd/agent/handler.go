package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type TestHandler struct {
}

func (h *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.Header.Get("name")
	fmt.Println("name:", name)

	//
	//go func() {
	//	//h.Conn.WriteMessage(websocket.TextMessage, []byte("ccc"))
	//	wr, err := h.Conn.NextWriter(websocket.TextMessage)
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//	_, err = wr.Write([]byte("ccc"))
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//	// 必须close， write不了
	//	wr.Close()
	//	fmt.Println("vvvvedwssa")
	//}()
	//
	//time.Sleep(2 * time.Second)

	_, err := w.Write([]byte(name))
	if err != nil {
		log.Fatalln(err)
	}

}


func ReverseProxyHandler(scheme, host string) http.Handler {
	reverseProxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Host:   host,
		Scheme: scheme,
	})

	reverseProxy.ErrorHandler = ErrHandler

	return reverseProxy
}

func ErrHandler(writer http.ResponseWriter, _ *http.Request, err error) {
	if err != nil {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte("reverse proxy:"+err.Error()))
		logrus.Error(err.Error())
	}
}