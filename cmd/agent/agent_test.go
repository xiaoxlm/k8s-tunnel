package main

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"testing"
	"time"
)

func SlowFunc1(conn *websocket.Conn) error {
	writer, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	_, err = writer.Write([]byte("sleep 3s"))
	writer.Close()
	return err

}

func SlowFunc2(conn *websocket.Conn) error {
	writer, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	_, err = writer.Write([]byte("sleep 5s"))
	writer.Close()
	return err
}

func TestAgent(t *testing.T) {
	t.Run("#read test", func(t *testing.T) {
		ctx := context.Background()
		a := NewAgent(&Option{GatewayHost: "127.0.0.1:9991"})
		err := a.Dial(context.Background(), "/read-test", nil)
		if err != nil {
			t.Fatal(err)
		}
		go a.SendPing(ctx)
		a.PingHandler()

		go func() {
			err = SlowFunc1(a.GetConn())
			if err != nil {
				t.Fatal(err)
			}
		}()

		go func() {
			err = SlowFunc2(a.GetConn())
			if err != nil {
				t.Fatal(err)
			}
		}()

		select {

		}

	})

	t.Run("#read-write", func(t *testing.T) {
		ctx := context.Background()
		a := NewAgent(&Option{GatewayHost: "127.0.0.1:9991"})
		err := a.Dial(context.Background(), "/test", nil)
		if err != nil {
			t.Fatal(err)
		}
		go a.SendPing(ctx)
		a.PingHandler()

		go func() {
			for {
				_, message, err := a.GetConn().ReadMessage()
				if err != nil {
					log.Println("read:", err)
					return
				}
				log.Printf("recv: %s", message)
			}
		}()

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for tk := range ticker.C {
			//cli.
			err = a.GetConn().WriteMessage(websocket.TextMessage, []byte("from client:"+tk.String()))
			if err != nil {
				fmt.Printf("client write message error. err:%v\n", err)
			}
		}
	})

	t.Run("#request", func(t *testing.T) {
		ctx := context.Background()
		a := NewAgent(&Option{
			AgentName: "huawei",
			GatewayHost: "127.0.0.1:9991"})

		path := fmt.Sprintf("/agents/%s/register", a.AgentName)

		err := a.Dial(context.Background(), path, nil)
		if err != nil {
			t.Fatal(err)
		}
		go a.SendPing(ctx)
		//a.PingHandler()

		for {
			err = a.HandleRequest()
			if err != nil {
				t.Fatal(err)
			}
		}

	})
}