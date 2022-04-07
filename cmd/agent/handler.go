package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func K8sReverseProxyHandler() (http.Handler, error) {
	config, err := GetRestConfig(true)
	if err != nil {
		return nil, err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Host:   strings.TrimPrefix(config.Host, "https://"),
		Scheme: "https",
	})

	transport, err := rest.TransportFor(config)
	if err != nil {
		return nil, err
	}

	reverseProxy.Transport = transport

	return reverseProxy, nil
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

func GetRestConfig(local bool) (*rest.Config, error) {
	var err error
	var config *rest.Config

	if local {
		var kubeconfig *string
		if home := homeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		//在 kubeconfig 中使用当前上下文环境，config 获取支持 url 和 path 方式
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return nil, err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	config.Timeout = time.Second * 10

	return config, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}