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
			agent := NewAgent(&Option{
				AgentName:   agentName,
				GatewayHost: gatewayHost,
			})
			agent.Serve()
		},
	}
	//rootCMD.AddCommand(cmd)

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}