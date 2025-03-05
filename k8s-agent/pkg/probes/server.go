package probes

import (
	"context"
	"net/http"

	"github.com/Sanskarzz/k8sgptclient/k8s-agent/pkg/server"
	"github.com/Sanskarzz/k8sgptclient/k8s-agent/pkg/server/handlers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewServer(addr string, mgr ctrl.Manager) server.ServerFunc {
	return func(ctx context.Context) error {
		logger := log.FromContext(ctx).WithName("probes")

		// create mux
		logger.Info("Creating new server mux")
		mux := http.NewServeMux()

		// register health check that verifies manager health
		logger.Info("Registering health check endpoint", "path", "/livez")
		mux.Handle("GET /livez", handlers.Healthy(func() bool {
			healthy := mgr.GetCache().WaitForCacheSync(ctx)
			logger.V(1).Info("Health check executed",
				"endpoint", "/livez",
				"status", healthy,
			)
			return healthy
		}))

		// register ready check
		logger.Info("Registering readiness check endpoint", "path", "/readyz")
		mux.Handle("GET /readyz", handlers.Ready(func() bool {
			ready := mgr.GetCache().WaitForCacheSync(ctx)
			logger.V(1).Info("Readiness check executed",
				"endpoint", "/readyz",
				"status", ready,
			)
			return ready
		}))

		// API endpoints
		// Accepts a YAML manifest and applies it to the cluster.
		logger.Info("Registering apply endpoint", "path", "/apply")
		mux.Handle("POST /apply", handlers.NewClientHandler(mgr.GetClient()).Apply())

		// Lists all pods in a specified namespace.
		logger.Info("Registering pods list endpoint", "path", "/pods")
		mux.Handle("GET /pods", handlers.NewClientHandler(mgr.GetClient()).ListPods())

		// Streams logs for a specific pod.
		logger.Info("Registering pod logs endpoint", "path", "/pods/{namespace}/{podName}/logs")
		mux.Handle("GET /pods/{namespace}/{podName}/logs", handlers.NewClientHandler(mgr.GetClient()).PodLogs())

		// Returns the status of a specific pod. including readiness and liveness probe results.
		logger.Info("Registering pod status endpoint", "path", "/pods/{namespace}/{podName}/status")
		mux.Handle("GET /pods/{namespace}/{podName}/status", handlers.NewClientHandler(mgr.GetClient()).PodStatus())

		// Get specific deployment yaml
		logger.Info("Registering deployment json endpoint", "path", "/deployment/{namespace}/{deploymentName}/yaml")
		mux.Handle("GET /deployment/{namespace}/{deploymentName}/yaml", handlers.NewClientHandler(mgr.GetClient()).DeploymentYaml())

		// Get specific pod yaml
		logger.Info("Registering pod json endpoint", "path", "/pod/{namespace}/{podName}/yaml")
		mux.Handle("GET /pod/{namespace}/{podName}/yaml", handlers.NewClientHandler(mgr.GetClient()).PodYaml())
		// create server
		s := &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		// run server
		return server.RunHttp(ctx, s, "", "")
	}
}
