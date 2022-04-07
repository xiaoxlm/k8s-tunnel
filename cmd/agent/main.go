package main

import (
	"github.com/spf13/cobra"
)

func main() {
	//rootCMD := &cobra.Command{}
	cmd := &cobra.Command{
		Use: "",
		Run: func(cmd *cobra.Command, args []string) {
			agentName := "huawei" // args[0]
			gatewayHost := "127.0.0.1:9991" // args[1]
			endpointURL := "http://127.0.0.1:80" // args[2] 反向代理端
			agent := NewAgent(&Option{
				AgentName:   agentName,
				GatewayHost: gatewayHost,
				EndpointURL: endpointURL,
			})
			agent.Serve()
		},
	}
	//rootCMD.AddCommand(cmd)

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}