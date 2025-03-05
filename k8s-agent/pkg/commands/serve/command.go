package serve

import (
	agent "github.com/Sanskarzz/k8sgptclient/k8s-agent/pkg/commands/serve/agent"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Run k8sgptclient servers",
	}
	// command to start the k8sagent
	command.AddCommand(agent.Command())
	// add more commands here like server
	return command
}
