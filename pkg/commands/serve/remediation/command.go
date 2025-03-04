package remediation

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Sanskarzz/k8sgptclient/pkg/ai"
	"github.com/Sanskarzz/k8sgptclient/pkg/gptscript"
	"github.com/fatih/color"
	openapi_v2 "github.com/google/gnostic/openapiv2"
	"github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	"github.com/k8sgpt-ai/k8sgpt/pkg/cache"
	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RemediationServer struct {
	analyzer *Analysis
	agentURL string
	apiKey   string
}

type Analysis struct {
	Context            context.Context
	Filters            []string
	Client             *kubernetes.Client
	Language           string
	AIClient           ai.IAI
	Results            []common.Result
	Errors             []string
	Namespace          string
	LabelSelector      string
	Cache              cache.ICache
	Explain            bool
	MaxConcurrency     int
	AnalysisAIProvider string // The name of the AI Provider used for this analysis
	WithDoc            bool
	WithStats          bool
	Stats              []common.AnalysisStats
}

const (
	defaultBackend = "openai"
	defaultModel   = "o3-mini"
)

func NewAnalysis(
	backend string,
	language string,
	filters []string,
	namespace string,
	labelSelector string,
	noCache bool,
	explain bool,
	maxConcurrency int,
	withDoc bool,
	interactiveMode bool,
	httpHeaders []string,
	withStats bool,
	configFile string,
) (*Analysis, error) {
	log.Printf("Reading config file: %s", configFile)
	// Read config file
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Extract AI configuration with correct structure
	var config struct {
		AI struct {
			Providers       []ai.AIProvider `mapstructure:"providers"`
			DefaultProvider string          `mapstructure:"defaultprovider"`
		} `mapstructure:"ai"`
	}

	// Extract AI config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Use default provider if backend not specified
	if backend == "" {
		backend = "openai"
		log.Printf("Using default backend: %s", backend)
	}

	log.Printf("Loaded AI config: DefaultProvider=%s, ProvidersCount=%d",
		config.AI.DefaultProvider, len(config.AI.Providers))

	// Find the provider configuration
	var provider *ai.AIProvider
	for _, p := range config.AI.Providers {
		log.Printf("Found provider in config: %s", p.Name)
		if p.Name == backend {
			provider = &p
			log.Printf("Found provider configuration: name=%s, model=%s", p.Name, p.Model)
			break
		}
	}
	if provider == nil {
		return nil, fmt.Errorf("provider %s not found in config", backend)
	}

	// Initialize AI client with the provider configuration
	log.Printf("Configuring AI client with provider: %s", provider.Name)
	aiClient := ai.NewClient(backend)
	if err := aiClient.Configure(provider); err != nil {
		return nil, fmt.Errorf("failed to configure AI client: %v", err)
	}

	kubecontext := viper.GetString("kubecontext")
	kubeconfig := viper.GetString("kubeconfig")
	// Rest of the initialization...
	client, err := kubernetes.NewClient(kubecontext, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("initialising kubernetes client: %w", err)
	}

	cache, err := cache.GetCacheConfiguration()
	if err != nil {
		return nil, err
	}
	if noCache {
		cache.DisableCache()
	}

	return &Analysis{
		Context:            context.Background(),
		Filters:            filters,
		Client:             client,
		Language:           language,
		AIClient:           aiClient,
		Namespace:          namespace,
		LabelSelector:      labelSelector,
		Cache:              cache,
		Explain:            explain,
		MaxConcurrency:     maxConcurrency,
		WithDoc:            withDoc,
		WithStats:          withStats,
		AnalysisAIProvider: backend,
	}, nil
}

func (a *Analysis) RunAnalysis() {
	activeFilters := viper.GetStringSlice("active_filters")

	coreAnalyzerMap, analyzerMap := analyzer.GetAnalyzerMap()

	// we get the openapi schema from the server only if required by the flag "with-doc"
	openapiSchema := &openapi_v2.Document{}
	if a.WithDoc {
		var openApiErr error

		openapiSchema, openApiErr = a.Client.Client.Discovery().OpenAPISchema()
		if openApiErr != nil {
			a.Errors = append(a.Errors, fmt.Sprintf("[KubernetesDoc] %s", openApiErr))
		}
	}

	analyzerConfig := common.Analyzer{
		Client:        a.Client,
		Context:       a.Context,
		Namespace:     a.Namespace,
		LabelSelector: a.LabelSelector,
		AIClient:      a.AIClient,
		OpenapiSchema: openapiSchema,
	}

	semaphore := make(chan struct{}, a.MaxConcurrency)
	var wg sync.WaitGroup
	var mutex sync.Mutex
	// if there are no filters selected and no active_filters then run coreAnalyzer
	if len(a.Filters) == 0 && len(activeFilters) == 0 {
		for name, analyzer := range coreAnalyzerMap {
			wg.Add(1)
			semaphore <- struct{}{}
			go a.executeAnalyzer(analyzer, name, analyzerConfig, semaphore, &wg, &mutex)

		}
		wg.Wait()
		return
	}
	// if the filters flag is specified
	if len(a.Filters) != 0 {
		for _, filter := range a.Filters {
			if analyzer, ok := analyzerMap[filter]; ok {
				semaphore <- struct{}{}
				wg.Add(1)
				go a.executeAnalyzer(analyzer, filter, analyzerConfig, semaphore, &wg, &mutex)
			} else {
				a.Errors = append(a.Errors, fmt.Sprintf("\"%s\" filter does not exist. Please run k8sgpt filters list.", filter))
			}
		}
		wg.Wait()
		return
	}

	// use active_filters
	for _, filter := range activeFilters {
		if analyzer, ok := analyzerMap[filter]; ok {
			semaphore <- struct{}{}
			wg.Add(1)
			go a.executeAnalyzer(analyzer, filter, analyzerConfig, semaphore, &wg, &mutex)
		}
	}
	wg.Wait()
}

func (a *Analysis) executeAnalyzer(analyzer common.IAnalyzer, filter string, analyzerConfig common.Analyzer, semaphore chan struct{}, wg *sync.WaitGroup, mutex *sync.Mutex) {
	defer wg.Done()

	var startTime time.Time
	var elapsedTime time.Duration

	// Start the timer
	if a.WithStats {
		startTime = time.Now()
	}

	// Run the analyzer
	results, err := analyzer.Analyze(analyzerConfig)

	// Measure the time taken
	if a.WithStats {
		elapsedTime = time.Since(startTime)
	}
	stat := common.AnalysisStats{
		Analyzer:     filter,
		DurationTime: elapsedTime,
	}

	mutex.Lock()
	defer mutex.Unlock()

	if err != nil {
		if a.WithStats {
			a.Stats = append(a.Stats, stat)
		}
		a.Errors = append(a.Errors, fmt.Sprintf("[%s] %s", filter, err))
	} else {
		if a.WithStats {
			a.Stats = append(a.Stats, stat)
		}
		a.Results = append(a.Results, results...)
	}
	<-semaphore
}

func (a *Analysis) GetAIResults(output string, anonymize bool) error {
	if len(a.Results) == 0 {
		return nil
	}

	var bar *progressbar.ProgressBar
	if output != "json" {
		bar = progressbar.Default(int64(len(a.Results)))
	}

	for index, analysis := range a.Results {
		var texts []string

		for _, failure := range analysis.Error {
			if anonymize {
				for _, s := range failure.Sensitive {
					failure.Text = util.ReplaceIfMatch(failure.Text, s.Unmasked, s.Masked)
				}
			}
			texts = append(texts, failure.Text)
		}

		promptTemplate := ai.PromptMap["default"]
		// If the resource `Kind` comes from an "integration plugin",
		// maybe a customized prompt template will be involved.
		if prompt, ok := ai.PromptMap[analysis.Kind]; ok {
			promptTemplate = prompt
		}
		result, err := a.getAIResultForSanitizedFailures(texts, promptTemplate)
		if err != nil {
			// FIXME: can we avoid checking if output is json multiple times?
			//   maybe implement the progress bar better?
			if output != "json" {
				_ = bar.Exit()
			}

			// Check for exhaustion.
			if strings.Contains(err.Error(), "status code: 429") {
				return fmt.Errorf("exhausted API quota for AI provider %s: %v", a.AIClient.GetName(), err)
			}
			return fmt.Errorf("failed while calling AI provider %s: %v", a.AIClient.GetName(), err)
		}

		if anonymize {
			for _, failure := range analysis.Error {
				for _, s := range failure.Sensitive {
					result = strings.ReplaceAll(result, s.Masked, s.Unmasked)
				}
			}
		}

		analysis.Details = result
		if output != "json" {
			_ = bar.Add(1)
		}
		a.Results[index] = analysis
	}
	return nil
}

func (a *Analysis) getAIResultForSanitizedFailures(texts []string, promptTmpl string) (string, error) {
	inputKey := strings.Join(texts, " ")
	// Check for cached data.
	// TODO(bwplotka): This might depend on model too (or even other client configuration pieces), fix it in later PRs.
	cacheKey := util.GetCacheKey(a.AIClient.GetName(), a.Language, inputKey)

	if !a.Cache.IsCacheDisabled() && a.Cache.Exists(cacheKey) {
		response, err := a.Cache.Load(cacheKey)
		if err != nil {
			return "", err
		}

		if response != "" {
			output, err := base64.StdEncoding.DecodeString(response)
			if err == nil {
				return string(output), nil
			}
			color.Red("error decoding cached data; ignoring cache item: %v", err)
		}
	}

	// Process template.
	prompt := fmt.Sprintf(strings.TrimSpace(promptTmpl), a.Language, inputKey)
	response, err := a.AIClient.GetCompletion(a.Context, prompt)
	if err != nil {
		return "", err
	}

	if err = a.Cache.Store(cacheKey, base64.StdEncoding.EncodeToString([]byte(response))); err != nil {
		color.Red("error storing value to cache; value won't be cached: %v", err)
	}
	return response, nil
}

func (a *Analysis) Close() {
	if a.AIClient == nil {
		return
	}
	a.AIClient.Close()
}

func (s *RemediationServer) runAnalysis() {
	log.Println("Starting k8sgpt analysis...")

	// Run the analysis
	s.analyzer.RunAnalysis()

	if len(s.analyzer.Errors) > 0 {
		log.Printf("Errors during analysis: %v", s.analyzer.Errors)
	}

	if s.analyzer.Explain {
		if err := s.analyzer.GetAIResults("text", false); err != nil {
			log.Printf("Error getting AI results: %v", err)
		}
	}

	// Initialize remediation generator
	remediator, err := gptscript.NewRemediationGenerator(s.apiKey, s.agentURL)
	if err != nil {
		log.Printf("Failed to initialize remediation generator: %v", err)
	}
	defer remediator.Close()

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
			log.Printf("\nError: %s\n", failure.Text)
		}

		if result.Details != "" {
			log.Printf("\nAnalysis Details: %s\n", result.Details)
		}

		// Generate remediation YAML
		remediationYAML, err := remediator.GenerateRemediation(context.Background(), result)
		if err != nil {
			log.Printf("Failed to generate remediation: %v", err)
			continue
		}

		log.Printf("Generated remediation YAML:\n%s\n", remediationYAML)
	}
}

func Command() *cobra.Command {
	var (
		httpAddress    string
		agentURL       string
		backend        string
		model          string
		password       string
		apiKey         string
		language       string
		filters        []string
		namespace      string
		labelSelector  string
		noCache        bool
		explain        bool
		maxConcurrency int
		withDoc        bool
		withStats      bool
		configFile     string
	)

	command := &cobra.Command{
		Use:   "remediation-server",
		Short: "Run k8sgptclient remediation-server",
		RunE: func(cmd *cobra.Command, args []string) error {

			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()

			log.Printf("Starting remediation server on %s", httpAddress)
			log.Printf("K8s agent URL: %s", agentURL)

			for range ticker.C {
				// Set up viper to use the config file
				viper.SetConfigFile(configFile)
				if err := viper.ReadInConfig(); err != nil {
					return fmt.Errorf("failed to read config file: %v", err)
				}

				// Initialize analyzer with all parameters
				analysis, err := NewAnalysis(
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
					configFile,
				)
				if err != nil {
					return err
				}

				server := &RemediationServer{
					analyzer: analysis,
					agentURL: agentURL,
					apiKey:   apiKey,
				}

				// Start analysis in background
				go server.runAnalysis()
			}

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
	command.Flags().StringVar(&apiKey, "api-key", "", "Backend AI password/key")
	command.Flags().StringVar(&configFile, "config", "/root/.config/k8sgpt/k8sgpt.yaml", "Path to k8sgpt config file")

	// Mark required flags
	command.MarkFlagRequired("backend")

	return command
}
