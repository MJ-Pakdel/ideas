package main

import (
	"context"
	"expvar"
	"flag"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/example/idaes/internal/config"
	"github.com/example/idaes/internal/subsystems"
)

// Command line flags for IDAES subsystem
var (
	// Web subsystem flags
	host         = flag.String("host", "localhost", "The host to which to bind the web server")
	webPort      = flag.Int("port", 8080, "The port on which to run the web server")
	readTimeout  = flag.Duration("read_timeout", time.Second*15, "The read timeout for the web server")
	writeTimeout = flag.Duration("write_timeout", time.Second*15, "The write timeout for the web server")
	idleTimeout  = flag.Duration("idle_timeout", time.Second*60, "The idle timeout for the web server")
	staticDir    = flag.String("static_dir", "./web/static", "The directory for static files")
	templateDir  = flag.String("template_dir", "./web/templates", "The directory for HTML templates")

	// External service URLs
	chromaURL = flag.String("chroma-url", "http://localhost:8000", "URL for ChromaDB service")
	ollamaURL = flag.String("ollama-url", "http://localhost:11434", "URL for Ollama service")

	// ChromaDB configuration
	chromaTenant   = flag.String("chroma-tenant", "default_tenant", "ChromaDB tenant name")
	chromaDatabase = flag.String("chroma-database", "default_database", "ChromaDB database name")

	// IDAES subsystem flags
	requestBuffer      = flag.Int("request_buffer", 100, "Buffer size for IDAES request channel")
	responseBuffer     = flag.Int("response_buffer", 100, "Buffer size for IDAES response channel")
	workerCount        = flag.Int("worker_count", 1, "Number of analysis workers in IDAES subsystem")  // FIXED: Force single worker to eliminate pipeline coordination issues
	maxConcurrentDocs  = flag.Int("max-concurrent-docs", 1, "Maximum concurrent documents to process") // FIXED: Force single document processing
	requestTimeout     = flag.Duration("request_timeout", time.Minute*5, "Timeout for individual analysis requests")
	parallelProcessing = flag.Bool("parallel-processing", false, "Enable parallel processing") // FIXED: Disable parallel processing

	// Dual Ollama Model Configuration
	intelligenceModel = flag.String("intelligence-model", "llama3.2:1b", "Ollama model for entity extraction and analysis")
	embeddingModel    = flag.String("embedding-model", "nomic-embed-text", "Ollama model for text embeddings")

	// LLM timeout configuration
	llmTimeout       = flag.Duration("llm-timeout", time.Second*60, "Timeout for LLM requests (generation and embedding)")
	llmRetryAttempts = flag.Int("llm-retry-attempts", 3, "Number of retry attempts for LLM requests")

	// System flags
	enableWeb       = flag.Bool("enable_web", true, "Enable the web subsystem")
	enableIDaes     = flag.Bool("enable_idaes", true, "Enable the IDAES subsystem")
	enableDisplay   = flag.Bool("enable-display", false, "Enable display functionality")
	enableGPIO      = flag.Bool("enable-gpio", false, "Enable GPIO functionality")
	enablePprof     = flag.Bool("enable_pprof", false, "Enable pprof profiling endpoints")
	shutdownTimeout = flag.Duration("shutdown_timeout", time.Second*30, "Graceful shutdown timeout")
	debugLogging    = flag.Bool("debug", true, "Enable debug logging")
)

// Application metrics
var (
	// Application-specific metrics
	appMetrics = struct {
		startTime              *expvar.Int
		documentsProcessed     *expvar.Int
		documentsUploaded      *expvar.Int
		analysisRequestsTotal  *expvar.Int
		analysisRequestsActive *expvar.Int
		errorCount             *expvar.Int
		avgProcessingTime      *expvar.Float
		activeGoroutines       *expvar.Int
		heapSize               *expvar.Int
		systemUptime           *expvar.Int
	}{
		startTime:              expvar.NewInt("app.start_time"),
		documentsProcessed:     expvar.NewInt("app.documents_processed_total"),
		documentsUploaded:      expvar.NewInt("app.documents_uploaded_total"),
		analysisRequestsTotal:  expvar.NewInt("app.analysis_requests_total"),
		analysisRequestsActive: expvar.NewInt("app.analysis_requests_active"),
		errorCount:             expvar.NewInt("app.errors_total"),
		avgProcessingTime:      expvar.NewFloat("app.avg_processing_time_seconds"),
		activeGoroutines:       expvar.NewInt("runtime.goroutines"),
		heapSize:               expvar.NewInt("runtime.heap_bytes"),
		systemUptime:           expvar.NewInt("app.uptime_seconds"),
	}
)

func updateRuntimeMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	appMetrics.activeGoroutines.Set(int64(runtime.NumGoroutine()))
	appMetrics.heapSize.Set(int64(m.HeapAlloc))
	appMetrics.systemUptime.Set(int64(time.Since(time.Unix(appMetrics.startTime.Value(), 0)).Seconds()))
}

func main() {
	flag.Parse()
	ctx := context.Background()

	// Initialize metrics
	appMetrics.startTime.Set(time.Now().Unix())

	// Configure logging level
	if *debugLogging {
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
		handler := slog.NewTextHandler(os.Stdout, opts)
		slog.SetDefault(slog.New(handler))
		slog.DebugContext(ctx, "Debug logging enabled")
	}

	// Debug flag values
	slog.DebugContext(ctx, "Flag values", "enablePprof", *enablePprof, "enableWeb", *enableWeb, "enableIDaes", *enableIDaes)

	// Log configuration for debugging
	slog.InfoContext(ctx, "Configuration",
		"chroma_url", *chromaURL,
		"chroma_tenant", *chromaTenant,
		"chroma_database", *chromaDatabase,
		"ollama_url", *ollamaURL,
		"intelligence_model", *intelligenceModel,
		"embedding_model", *embeddingModel,
		"webui_port", *webPort,
		"webui_host", *host,
		"workers", *workerCount,
		"max_concurrent_docs", *maxConcurrentDocs,
		"parallel_processing", *parallelProcessing,
		"enable_web", *enableWeb,
		"enable_idaes", *enableIDaes,
		"enable_display", *enableDisplay,
		"enable_gpio", *enableGPIO,
		"enable_pprof", *enablePprof)

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create error group for managing subsystems
	eg, ctx := errgroup.WithContext(ctx)

	// Start pprof server if enabled
	if *enablePprof {
		pprofServer := &http.Server{
			Addr: "0.0.0.0:6060",
		}

		slog.InfoContext(ctx, "Starting pprof server", "addr", pprofServer.Addr)

		eg.Go(func() error {
			if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.ErrorContext(ctx, "pprof server failed", "error", err)
				return err
			}
			return nil
		})

		// Handle pprof server shutdown
		eg.Go(func() error {
			<-ctx.Done()
			slog.InfoContext(ctx, "Shutting down pprof server")
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, *shutdownTimeout)
			defer shutdownCancel()
			return pprofServer.Shutdown(shutdownCtx)
		})

		// Start metrics server (combined with pprof)
		metricsServer := &http.Server{
			Addr: "0.0.0.0:8082",
		}

		// Set up metrics endpoints
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/debug/vars", expvar.Handler()) // expvar metrics
		metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			updateRuntimeMetrics()
			w.Header().Set("Content-Type", "application/json")
			expvar.Handler().ServeHTTP(w, r)
		})
		metricsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			updateRuntimeMetrics()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		})
		metricsServer.Handler = metricsMux

		slog.InfoContext(ctx, "Starting metrics server", "addr", metricsServer.Addr)

		eg.Go(func() error {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.ErrorContext(ctx, "metrics server failed", "error", err)
				return err
			}
			return nil
		})

		// Handle metrics server shutdown
		eg.Go(func() error {
			<-ctx.Done()
			slog.InfoContext(ctx, "Shutting down metrics server")
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, *shutdownTimeout)
			defer shutdownCancel()
			return metricsServer.Shutdown(shutdownCtx)
		})

		// Start runtime metrics updater
		eg.Go(func() error {
			ticker := time.NewTicker(10 * time.Second) // Update every 10 seconds
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					updateRuntimeMetrics()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	var idaesSubsystem *subsystems.IDaesSubsystem
	var webSubsystem *subsystems.WebSubsystem
	var hardwareSubsystem *subsystems.HardwareSubsystem

	// Start IDAES subsystem if enabled
	if *enableIDaes {
		slog.InfoContext(ctx, "Initializing IDAES subsystem")

		idaesConfig := &subsystems.IDaesConfig{
			AnalysisConfig:    nil, // Use default
			RequestBuffer:     *requestBuffer,
			ResponseBuffer:    *responseBuffer,
			WorkerCount:       *workerCount,
			RequestTimeout:    *requestTimeout,
			ShutdownTimeout:   *shutdownTimeout,
			ChromaURL:         *chromaURL,
			ChromaTenant:      *chromaTenant,
			ChromaDatabase:    *chromaDatabase,
			OllamaURL:         *ollamaURL,
			IntelligenceModel: *intelligenceModel,
			EmbeddingModel:    *embeddingModel,
			LLMTimeout:        *llmTimeout,
			LLMRetryAttempts:  *llmRetryAttempts,
		}

		// Note: ChromaURL and OllamaURL are now properly passed to the subsystem config

		var err error
		idaesSubsystem, err = subsystems.NewIDaesSubsystem(ctx, idaesConfig)
		if err != nil {
			slog.ErrorContext(ctx, "CRITICAL: Failed to create IDAES subsystem - application cannot function without proper ChromaDB/Ollama connectivity",
				"error", err,
				"chroma_url", *chromaURL,
				"ollama_url", *ollamaURL)
			os.Exit(1)
		}
		slog.InfoContext(ctx, "IDAES subsystem initialized successfully")

		// Start IDAES subsystem
		eg.Go(func() error {
			if err := idaesSubsystem.Start(); err != nil {
				slog.ErrorContext(ctx, "IDAES subsystem failed to start", "error", err)
				return err
			}

			// Wait for shutdown signal
			<-ctx.Done()

			slog.InfoContext(ctx, "Shutting down IDAES subsystem")
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, *shutdownTimeout)
			defer shutdownCancel()

			done := make(chan error, 1)
			go func() {
				done <- idaesSubsystem.Stop()
			}()

			select {
			case err := <-done:
				if err != nil {
					slog.ErrorContext(ctx, "IDAES subsystem shutdown error", "error", err)
				}
				return err
			case <-shutdownCtx.Done():
				slog.WarnContext(ctx, "IDAES subsystem shutdown timeout")
				return shutdownCtx.Err()
			}
		})
	}

	// Start web subsystem if enabled
	if *enableWeb {
		if idaesSubsystem == nil {
			log.Fatalf("Web subsystem requires IDAES subsystem to be enabled")
		}

		slog.InfoContext(ctx, "Initializing web subsystem")

		webConfig := &subsystems.WebSubsystemConfig{
			ServerConfig: &config.ServerConfig{
				Host:         *host,
				Port:         *webPort,
				ReadTimeout:  *readTimeout,
				WriteTimeout: *writeTimeout,
				IdleTimeout:  *idleTimeout,
				StaticDir:    *staticDir,
				TemplateDir:  *templateDir,
			},
			SubscriberID: "web_subsystem_main",
		}

		var err error
		webSubsystem, err = subsystems.NewWebSubsystem(ctx, webConfig, idaesSubsystem)
		if err != nil {
			log.Fatalf("Failed to create web subsystem: %v", err)
		}

		// Start web subsystem
		eg.Go(func() error {
			if err := webSubsystem.Start(); err != nil {
				slog.ErrorContext(ctx, "Web subsystem failed to start", "error", err)
				return err
			}

			// Wait for shutdown signal
			<-ctx.Done()

			slog.InfoContext(ctx, "Shutting down web subsystem")
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, *shutdownTimeout)
			defer shutdownCancel()

			done := make(chan error, 1)
			go func() {
				done <- webSubsystem.Stop()
			}()

			select {
			case err := <-done:
				if err != nil {
					slog.ErrorContext(ctx, "Web subsystem shutdown error", "error", err)
				}
				return err
			case <-shutdownCtx.Done():
				slog.WarnContext(ctx, "Web subsystem shutdown timeout")
				return shutdownCtx.Err()
			}
		})
	}

	// Start hardware subsystem if enabled (display/GPIO for Raspberry Pi)
	if *enableDisplay || *enableGPIO {
		slog.InfoContext(ctx, "Initializing hardware subsystem",
			"display_enabled", *enableDisplay,
			"gpio_enabled", *enableGPIO)

		hardwareConfig := &subsystems.HardwareConfig{
			EnableDisplay:          *enableDisplay,
			EnableGPIO:             *enableGPIO,
			DisplayConfig:          nil, // Use defaults
			GPIOConfig:             nil, // Use defaults
			EnableSystemMonitoring: true,
			MonitoringInterval:     time.Second * 10,
			GPIOPreset:             "basic_led",
		}

		var err error
		hardwareSubsystem, err = subsystems.NewHardwareSubsystem(ctx, hardwareConfig)
		if err != nil {
			slog.WarnContext(ctx, "Failed to create hardware subsystem", "error", err)
			// Don't exit - hardware is optional
		} else {
			slog.InfoContext(ctx, "Hardware subsystem initialized successfully")

			// Start hardware subsystem
			eg.Go(func() error {
				if err := hardwareSubsystem.Start(ctx); err != nil {
					slog.ErrorContext(ctx, "Hardware subsystem failed to start", "error", err)
					return err
				}

				// Wait for shutdown signal
				<-ctx.Done()

				slog.InfoContext(ctx, "Shutting down hardware subsystem")
				shutdownCtx, shutdownCancel := context.WithTimeout(ctx, *shutdownTimeout)
				defer shutdownCancel()

				done := make(chan error, 1)
				go func() {
					done <- hardwareSubsystem.Stop(shutdownCtx)
				}()

				select {
				case err := <-done:
					if err != nil {
						slog.ErrorContext(ctx, "Hardware subsystem shutdown error", "error", err)
					}
					return err
				case <-shutdownCtx.Done():
					slog.WarnContext(ctx, "Hardware subsystem shutdown timeout")
					return shutdownCtx.Err()
				}
			})

			// Show startup message on display
			if *enableDisplay {
				go func() {
					time.Sleep(time.Second * 2) // Wait for display to initialize
					_ = hardwareSubsystem.ShowMessage(ctx, "IDAES System Started", time.Second*5)
				}()
			}
		}
	}

	// Handle shutdown signals
	eg.Go(func() error {
		select {
		case sig := <-sigChan:
			slog.InfoContext(ctx, "Received shutdown signal", "signal", sig)
			cancel()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Log startup completion
	slog.InfoContext(ctx, "System startup complete",
		"idaes_enabled", *enableIDaes,
		"web_enabled", *enableWeb,
		"display_enabled", *enableDisplay,
		"gpio_enabled", *enableGPIO,
		"host", *host,
		"port", *webPort)

	// Wait for all subsystems to complete
	if err := eg.Wait(); err != nil && err != context.Canceled {
		slog.ErrorContext(ctx, "System error", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "System shutdown complete")
}
