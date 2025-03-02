package probes

import (
	"context"
	"net/http"

	"github.com/Sanskarzz/k8sgptclient/pkg/server"
	"github.com/Sanskarzz/k8sgptclient/pkg/server/handlers"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewServer(addr string, mgr ctrl.Manager) server.ServerFunc {
	return func(ctx context.Context) error {
		// create mux
		mux := http.NewServeMux()
		// register health check that verifies manager health
		mux.Handle("GET /livez", handlers.Healthy(func() bool {
			// Check if manager is healthy
			return mgr.GetCache().WaitForCacheSync(ctx)
		}))
		// register ready check
		mux.Handle("GET /readyz", handlers.Ready(func() bool {
			// Check if manager is ready
			return mgr.GetCache().WaitForCacheSync(ctx)
		}))

		// API endpoints
		// Accepts a YAML manifest and applies it to the cluster.
		mux.Handle("POST /apply", handlers.NewClientHandler(mgr.GetClient()).Apply())
		// Lists all pods in a specified namespace.
		mux.Handle("GET /pods", handlers.NewClientHandler(mgr.GetClient()).ListPods())
		// Streams logs for a specific pod.
		mux.Handle("GET /pods/{namespace}/{podName}/logs", handlers.NewClientHandler(mgr.GetClient()).PodLogs())
		// Returns the status of a specific pod. including readiness and liveness probe results.
		mux.Handle("GET /pods/{namespace}/{podName}/status", handlers.NewClientHandler(mgr.GetClient()).PodStatus())
		// create server
		s := &http.Server{
			Addr:    addr,
			Handler: mux,
		}
		// run server
		return server.RunHttp(ctx, s, "", "")
	}
}
