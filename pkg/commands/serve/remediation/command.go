package remediation

import (
	"context"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type RemediationServer struct {
	client dynamic.Interface
}

func (s *RemediationServer) watchResults() {
	// Define the GVR for K8sGPT Results
	resultsGVR := schema.GroupVersionResource{
		Group:    "core.k8sgpt.ai",
		Version:  "v1alpha1",
		Resource: "results",
	}

	// Watch for Results in k8sgpt-operator-system namespace
	watcher, err := s.client.Resource(resultsGVR).
		Namespace("k8sgpt-operator-system").
		Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error watching results: %v", err)
	}

	log.Println("Starting to watch for K8sGPT results...")
	for event := range watcher.ResultChan() {
		log.Printf("Received event: %v", event)
		// Here we'll process the Result and generate remediation
	}
}

func Command() *cobra.Command {
	var httpAddress string
	// var k8sAgentURL string
	command := &cobra.Command{
		Use:   "remediation-server",
		Short: "Run k8sgptclient remediation-server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get kubernetes client config
			config, err := rest.InClusterConfig()
			if err != nil {
				return err
			}

			// Create dynamic client
			client, err := dynamic.NewForConfig(config)
			if err != nil {
				return err
			}

			server := &RemediationServer{
				client: client,
			}

			// Start watching results in a goroutine
			go server.watchResults()

			// Keep the HTTP server for compatibility
			http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			log.Printf("Starting remediation server on %s", httpAddress)
			return http.ListenAndServe(httpAddress, nil)
		},
	}

	command.Flags().StringVar(&httpAddress, "http-address", ":9090", "The address the remediation server binds to")
	// command.Flags().StringVar(&k8sAgentURL, "k8s-agent-url", "http://localhost:8080", "URL of the k8s-agent service")
	return command
}
