package utils

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"testing"
)

func Test(t *testing.T) {
	t.Run("#BuildRESP", func(t *testing.T) {
		resp, err := BuildResponse("hello, world")
		if err != nil {
			t.Fatal(err)
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Print("resp:",string(b))

		u, _ := url.Parse("https://www.baidu.com")
		fmt.Println(u)
	})
}
