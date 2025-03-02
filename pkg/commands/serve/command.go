package serve

import (
	server "github.com/Sanskarzz/k8sgptclient/pkg/commands/serve/server"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Run k8sgptclient servers",
	}
	command.AddCommand(server.Command())
	// add more commands here like server
	return command
}
