package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"k8s-tunnel/pkg/log"
)

func init()  {
	log.LogInit("server", logrus.DebugLevel)
}

func main() {
	gw := NewGateway()

	ctx := context.Background()
	_ = gw.Serve(ctx)
}