package remediation

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/k8sgpt-ai/k8sgpt/pkg/ai"
	"github.com/k8sgpt-ai/k8sgpt/pkg/analysis"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RemediationServer struct {
	analyzer *analysis.Analysis
	agentURL string
}

// AIConfig represents the configuration for AI providers
type AIConfig struct {
	Providers []ai.AIProvider `mapstructure:"providers"`
}

const (
	defaultBackend = "openai"
	defaultModel   = "o3-mini"
)

func (s *RemediationServer) runAnalysis() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Use a for range loop to iterate over the ticker's channel
	for range ticker.C {
		log.Println("Starting k8sgpt analysis...")

		// Run the analysis
		s.analyzer.RunAnalysis()

		if len(s.analyzer.Errors) > 0 {
			log.Printf("Errors during analysis: %v", s.analyzer.Errors)
			continue
		}

		// Process results
		for _, result := range s.analyzer.Results {
			log.Printf("\nFound issue in resource:\n"+
				"Kind: %s\n"+
				"Name: %s\n"+
				"Parent: %s\n",
				result.Kind,
				result.Name,
				result.ParentObject,
			)

			// Print each error and its details
			for _, failure := range result.Error {
				log.Printf("Error: %s\n", failure.Text)
			}

			if result.Details != "" {
				log.Printf("Analysis Details: %s\n", result.Details)
			}

			// TODO: Generate remediation YAML using GPTScript
			// TODO: Send to k8s-agent at s.agentURL
		}

		// Optional: Print analysis stats if enabled
		if s.analyzer.WithStats {
			statsBytes := s.analyzer.PrintStats()
			log.Printf("Analysis Stats:\n%s", string(statsBytes))
		}
	}
}

func Command() *cobra.Command {
	var (
		httpAddress    string
		agentURL       string
		backend        string
		model          string
		password       string
		language       string
		filters        []string
		namespace      string
		labelSelector  string
		noCache        bool
		explain        bool
		maxConcurrency int
		withDoc        bool
		withStats      bool
		temperature    float32
	)

	// Declare configAI variable
	var configAI AIConfig

	command := &cobra.Command{
		Use:   "remediation-server",
		Short: "Run k8sgptclient remediation-server",
		RunE: func(cmd *cobra.Command, args []string) error {

			// Set up viper configuration
			configDir := filepath.Join(os.TempDir(), "k8sgptclient")
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %v", err)
			}

			configPath := filepath.Join(configDir, "config.yaml")
			viper.SetConfigFile(configPath)
			viper.SetConfigType("yaml")

			// Create config file if it doesn't exist
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				file, err := os.Create(configPath)
				if err != nil {
					return fmt.Errorf("failed to create config file: %v", err)
				}
				file.Close()
			}

			// Read the config file
			if err := viper.ReadInConfig(); err != nil {
				// It's okay if the config file doesn't exist
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					return fmt.Errorf("failed to read config file: %v", err)
				}
			}

			// First authenticate with AI provider
			if backend == "" {
				backend = "openai"
			}

			// Validate backend
			validBackend := false
			for _, b := range ai.Backends {
				if b == backend {
					validBackend = true
					break
				}
			}
			if !validBackend {
				return fmt.Errorf("invalid backend: %s. Accepted values are: %v", backend, ai.Backends)
			}

			// Create AI provider configuration
			provider := ai.AIProvider{
				Name:        backend,
				Model:       model,
				Password:    password,
				Temperature: temperature,
			}

			// Store configuration
			config := AIConfig{
				Providers: []ai.AIProvider{provider},
			}

			// Set configuration in viper
			viper.Set("ai", config)

			// Write the configuration to file
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("failed to write config file: %v", err)
			}

			configAI.Providers = append(configAI.Providers, provider)
			viper.Set("ai", configAI)
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("failed to write config file: %v", err)
			}

			// Initialize analyzer with all parameters
			analyzer, err := analysis.NewAnalysis(
				backend,
				language,
				filters,
				namespace,
				labelSelector,
				noCache,
				explain,
				maxConcurrency,
				withDoc,
				false,      // Interactive mode always false for server
				[]string{}, // No custom HTTP headers
				withStats,
			)
			if err != nil {
				return err
			}

			server := &RemediationServer{
				analyzer: analyzer,
				agentURL: agentURL,
			}

			// Start analysis in background
			go server.runAnalysis()

			log.Printf("Starting remediation server on %s", httpAddress)
			log.Printf("K8s agent URL: %s", agentURL)
			log.Printf("Analysis configuration:")
			log.Printf("- Backend: %s", backend)
			log.Printf("- Filters: %v", filters)
			log.Printf("- Namespace: %s", namespace)
			log.Printf("- Explain mode: %v", explain)

			return http.ListenAndServe(httpAddress, nil)
		},
	}

	// Add all required flags
	command.Flags().StringVar(&httpAddress, "http-address", ":9090", "The address the remediation server binds to")
	command.Flags().StringVar(&agentURL, "agent-url", "http://k8s-agent.k8sgptclient.svc.cluster.local:8080", "K8s agent service URL")
	command.Flags().StringVar(&backend, "backend", "openai", "AI backend to use (openai, azure, etc)")
	command.Flags().StringVar(&language, "language", "english", "Language for analysis output")
	command.Flags().StringSliceVar(&filters, "filters", []string{"Deployment", "Pod"}, "Resource types to analyze")
	command.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace to analyze (empty for all)")
	command.Flags().StringVar(&labelSelector, "selector", "", "Label selector to filter resources")
	command.Flags().BoolVar(&noCache, "no-cache", true, "Disable caching of analysis results")
	command.Flags().BoolVar(&explain, "explain", true, "Get detailed explanations")
	command.Flags().IntVar(&maxConcurrency, "max-concurrency", 10, "Maximum concurrent analyses")
	command.Flags().BoolVar(&withDoc, "with-doc", false, "Include documentation in results")
	command.Flags().BoolVar(&withStats, "with-stats", false, "Include statistics in results")
	command.Flags().StringVarP(&password, "password", "p", "OPENAI_API_KEY", "API key for the AI provider")
	command.Flags().StringVarP(&model, "model", "m", defaultModel, "Backend AI model")

	// Mark required flags
	command.MarkFlagRequired("backend")

	return command
}
