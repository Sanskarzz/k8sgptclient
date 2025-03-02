package agent

import (
	"context"

	"github.com/Sanskarzz/k8sgptclient/pkg/probes"
	"github.com/Sanskarzz/k8sgptclient/pkg/signals"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Command() *cobra.Command {
	var httpAddress string
	var kubeConfigOverrides clientcmd.ConfigOverrides
	command := &cobra.Command{
		Use:   "agent",
		Short: "Start k8sgptclient Serve Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// setup signals aware context
			return signals.Do(context.Background(), func(ctx context.Context) error {
				// track errors
				var httpErr, mgrErr error
				err := func(ctx context.Context) error {
					// create a kubernetes rest config
					kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
						clientcmd.NewDefaultClientConfigLoadingRules(),
						&kubeConfigOverrides,
					)
					config, err := kubeConfig.ClientConfig()
					if err != nil {
						return err
					}

					// create a manager
					mgr, err := ctrl.NewManager(config, ctrl.Options{
						Scheme: nil, // we'll use the default scheme
					})
					if err != nil {
						return err
					}

					// create a wait group
					var group wait.Group
					// wait all tasks in the group are over
					defer group.Wait()

					// create a cancellable context
					ctx, cancel := context.WithCancel(ctx)
					// start manager
					group.StartWithContext(ctx, func(ctx context.Context) {
						// cancel context at the end
						defer cancel()
						mgrErr = mgr.Start(ctx)
					})

					// create http server
					http := probes.NewServer(httpAddress, mgr)
					// run server
					group.StartWithContext(ctx, func(ctx context.Context) {
						// cancel context at the end
						defer cancel()
						httpErr = http.Run(ctx)
					})
					return nil
				}(ctx)
				return multierr.Combine(err, httpErr, mgrErr)
			})
		},
	}

	command.Flags().StringVar(&httpAddress, "http-address", ":8080", "Address to listen on")
	clientcmd.BindOverrideFlags(&kubeConfigOverrides, command.Flags(), clientcmd.RecommendedConfigOverrideFlags("kube-"))

	return command
}
