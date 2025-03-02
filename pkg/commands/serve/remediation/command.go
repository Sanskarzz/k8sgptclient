package remediation

import (
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "remediation-server",
		Short: "Run k8sgptclient remediation-server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
