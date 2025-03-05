package serve

import (
	remediation "github.com/Sanskarzz/k8sgptclient/k8sgpt-remediation/pkg/commands/serve/remediation"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Run k8sgptclient servers",
	}
	// command to start the remediation-server
	command.AddCommand(remediation.Command())
	// add more commands here like server
	return command
}
