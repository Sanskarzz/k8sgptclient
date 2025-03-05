package root

import (
	"github.com/Sanskarzz/k8sgptclient/k8sgpt-remediation/pkg/commands/serve"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	root := &cobra.Command{
		Use:   "k8sgptclient",
		Short: "k8sgptclient is a client for k8sgpt",
	}
	root.AddCommand(serve.Command())
	return root
}
