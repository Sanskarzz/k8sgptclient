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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Command() *cobra.Command {
	var httpAddress string
	var kubeConfigOverrides clientcmd.ConfigOverrides
	command := &cobra.Command{
		Use:   "agent",
		Short: "Start k8sgptclient Serve Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get logger for agent component
			logger := log.Log.WithName("agent")

			// Log startup information
			logger.Info("Starting k8sgptclient agent",
				"httpAddress", httpAddress,
			)

			// setup signals aware context
			return signals.Do(context.Background(), func(ctx context.Context) error {
				// track errors
				var httpErr, mgrErr error
				err := func(ctx context.Context) error {
					// create a kubernetes rest config
					logger.Info("Loading kubernetes configuration")
					kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
						clientcmd.NewDefaultClientConfigLoadingRules(),
						&kubeConfigOverrides,
					)
					config, err := kubeConfig.ClientConfig()
					if err != nil {
						logger.Error(err, "Failed to load kubernetes config")
						return err
					}

					// create a manager
					logger.Info("Creating controller manager")
					mgr, err := ctrl.NewManager(config, ctrl.Options{
						Scheme: nil, // we'll use the default scheme
						Logger: logger.WithName("manager"),
					})
					if err != nil {
						logger.Error(err, "Failed to create manager")
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
						logger.Info("Starting controller manager")
						mgrErr = mgr.Start(ctx)
						if mgrErr != nil {
							logger.Error(mgrErr, "Manager stopped with error")
						} else {
							logger.Info("Manager stopped gracefully")
						}
					})

					// create http server
					logger.Info("Creating HTTP server", "address", httpAddress)
					http := probes.NewServer(httpAddress, mgr)
					// run server
					group.StartWithContext(ctx, func(ctx context.Context) {
						// cancel context at the end
						defer cancel()
						logger.Info("Starting HTTP server")
						httpErr = http.Run(ctx)
						if httpErr != nil {
							logger.Error(httpErr, "HTTP server stopped with error")
						} else {
							logger.Info("HTTP server stopped gracefully")
						}
					})
					return nil
				}(ctx)

				// Combine errors if any occurred
				if finalErr := multierr.Combine(err, httpErr, mgrErr); finalErr != nil {
					logger.Error(finalErr, "Server stopped with errors")
					return finalErr
				}
				logger.Info("Server stopped gracefully")
				return nil
			})
		},
	}

	command.Flags().StringVar(&httpAddress, "http-address", ":8080", "Address to listen on")
	clientcmd.BindOverrideFlags(&kubeConfigOverrides, command.Flags(), clientcmd.RecommendedConfigOverrideFlags("kube-"))

	return command
}
