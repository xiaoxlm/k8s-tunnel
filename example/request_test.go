package example

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// 通过中心端，请求远端接口
func TestRequest(t *testing.T) {
	simpleClientRequest()
}

func simpleClientRequest() {
	// after server，agent run

	u := &url.URL{}
	u.Scheme = "http"
	// host 和 /proxies/huawei 是tunnel server 的path;
	u.Host = "127.0.0.1:9991"
	u.Path = "/proxies/huawei"
	// 反向代理端的接口path
	u.Path += "/test/hello"
	req := &http.Request{}
	req.URL = u

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, _ := ioutil.ReadAll(resp.Body)

	fmt.Println(string(b))
}


