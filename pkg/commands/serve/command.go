package serve

import (
	agent "github.com/Sanskarzz/k8sgptclient/pkg/commands/serve/agent"
	remediation "github.com/Sanskarzz/k8sgptclient/pkg/commands/serve/remediation"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Run k8sgptclient servers",
	}
	// command to start the k8sagent
	command.AddCommand(agent.Command())
	// command to start the remediation-server
	command.AddCommand(remediation.Command())
	// add more commands here like server
	return command
}
