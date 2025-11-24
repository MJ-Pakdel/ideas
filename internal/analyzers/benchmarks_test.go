package analyzers_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/types"
)

// BenchmarkSuite provides comprehensive performance benchmarks comparing
// mutex-based vs channel-based implementations across all phases
type BenchmarkSuite struct {
	// Test data
	documents []*types.Document
	requests  []*analyzers.AnalysisRequest
}

// NewBenchmarkSuite creates a new benchmark suite with test data
func NewBenchmarkSuite() *BenchmarkSuite {
	suite := &BenchmarkSuite{}
	suite.generateTestData()
	return suite
}

// generateTestData creates realistic test documents and requests
func (bs *BenchmarkSuite) generateTestData() {
	// Create test documents of varying sizes and complexity
	testContents := []string{
		// Small document
		"This is a small test document with minimal content for basic testing.",

		// Medium document
		"This research paper discusses machine learning algorithms and their applications. " +
			"According to Smith et al. (2020), neural networks show promising results. " +
			"The study by Johnson (2019) demonstrates significant improvements. " +
			"Key topics include artificial intelligence, deep learning, and data analysis.",

		// Large document
		generateLargeDocument(),

		// Complex document with many entities and citations
		generateComplexDocument(),
	}

	// Create documents
	bs.documents = make([]*types.Document, 0, len(testContents)*3)
	for i, content := range testContents {
		bs.documents = append(bs.documents, &types.Document{
			ID:       fmt.Sprintf("doc-%d", i),
			Content:  content,
			Metadata: map[string]any{"source": "test"},
		})
	}

	// Create analysis requests
	bs.requests = make([]*analyzers.AnalysisRequest, 0, len(bs.documents))
	for i, doc := range bs.documents {
		bs.requests = append(bs.requests, &analyzers.AnalysisRequest{
			RequestID: fmt.Sprintf("req-%d", i),
			Document:  doc,
			Priority:  1,
		})
	}
}

// generateLargeDocument creates a large document for performance testing
func generateLargeDocument() string {
	baseText := "This is a comprehensive research study examining the effects of artificial intelligence on modern society. " +
		"The methodology involves extensive data collection and analysis using state-of-the-art machine learning algorithms. " +
		"Results indicate significant improvements in efficiency and accuracy across various domains. " +
		"Key findings suggest that AI technologies will continue to revolutionize industries worldwide. "

	// Repeat to create a large document
	var builder strings.Builder
	for i := 0; i < 100; i++ {
		builder.WriteString(fmt.Sprintf("[Section %d] %s", i+1, baseText))
	}
	return builder.String()
}

// generateComplexDocument creates a document with many entities and citations for complex testing
func generateComplexDocument() string {
	return "Recent studies by Anderson et al. (2023) and Wang, Liu, and Chen (2022) demonstrate significant advances " +
		"in natural language processing. The research conducted at Stanford University (California, USA) and " +
		"MIT (Massachusetts Institute of Technology, Cambridge, MA) shows promising results. " +
		"Key researchers include Dr. Sarah Johnson (Harvard Medical School), Prof. Michael Brown (Oxford University), " +
		"and Dr. Lisa Zhang (Beijing University). Organizations like Google AI, Microsoft Research, and OpenAI " +
		"have contributed substantial resources. The collaboration between CERN (European Organization for Nuclear Research) " +
		"and various international institutions has yielded groundbreaking discoveries published in Nature, Science, " +
		"and the Journal of Artificial Intelligence Research."
}

// MockCancellableComponent for testing cancellation patterns
type MockCancellableComponent struct {
	name      string
	isHealthy bool
	shutdowns int
}

func (m *MockCancellableComponent) Name() string {
	return m.name
}

func (m *MockCancellableComponent) IsHealthy(_ context.Context) bool {
	return m.isHealthy
}

func (m *MockCancellableComponent) Shutdown(_ context.Context) error {
	m.shutdowns++
	m.isHealthy = false
	return nil
}

// ===============================
// Core Concurrency Benchmarks
// ===============================

// BenchmarkChannelCoordination tests channel-based coordination patterns
func BenchmarkChannelCoordination(b *testing.B) {
	concurrencyLevels := []int{1, 10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Workers-%d", concurrency), func(b *testing.B) {
			var memBefore, memAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Channel-based worker coordination
				workChan := make(chan int, 100)
				resultChan := make(chan int, concurrency)
				var wg sync.WaitGroup

				// Start workers
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						for work := range workChan {
							// Simulate work
							result := work * 2
							select {
							case resultChan <- result:
							default:
							}
						}
					}(j)
				}

				// Send work
				go func() {
					defer close(workChan)
					for k := 0; k < 100; k++ {
						workChan <- k
					}
				}()

				// Collect results
				results := 0
				timeout := time.After(time.Second)
			collectLoop:
				for {
					select {
					case <-resultChan:
						results++
						if results >= 100 {
							break collectLoop
						}
					case <-timeout:
						break collectLoop
					}
				}

				close(resultChan)
				wg.Wait()
			}

			b.StopTimer()
			runtime.GC()
			runtime.ReadMemStats(&memAfter)

			allocDiff := memAfter.TotalAlloc - memBefore.TotalAlloc
			b.ReportMetric(float64(allocDiff)/float64(b.N), "bytes/op")
			b.ReportMetric(float64(concurrency), "workers")
		})
	}
}

// BenchmarkMutexCoordination tests mutex-based coordination patterns
func BenchmarkMutexCoordination(b *testing.B) {
	concurrencyLevels := []int{1, 10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Workers-%d", concurrency), func(b *testing.B) {
			var memBefore, memAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Mutex-based worker coordination
				var mu sync.Mutex
				var results []int
				var wg sync.WaitGroup

				work := make([]int, 100)
				for j := range work {
					work[j] = j
				}

				// Start workers
				workIndex := 0
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							mu.Lock()
							if workIndex >= len(work) {
								mu.Unlock()
								break
							}
							w := work[workIndex]
							workIndex++
							mu.Unlock()

							// Simulate work
							result := w * 2

							mu.Lock()
							results = append(results, result)
							mu.Unlock()
						}
					}()
				}

				wg.Wait()
			}

			b.StopTimer()
			runtime.GC()
			runtime.ReadMemStats(&memAfter)

			allocDiff := memAfter.TotalAlloc - memBefore.TotalAlloc
			b.ReportMetric(float64(allocDiff)/float64(b.N), "bytes/op")
			b.ReportMetric(float64(concurrency), "workers")
		})
	}
}

// ===============================
// Performance Benchmarks
// ===============================

// BenchmarkPhase3_MetricsBroadcasting benchmarks metrics broadcasting performance
func BenchmarkPhase3_MetricsBroadcasting(b *testing.B) {
	subscribers := []int{1, 10, 50}
	events := []int{100, 1000}
	ctx := b.Context()
	for _, numSubscribers := range subscribers {
		for _, numEvents := range events {
			b.Run(fmt.Sprintf("Subscribers-%d-Events-%d", numSubscribers, numEvents), func(b *testing.B) {
				var memBefore, memAfter runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&memBefore)

				b.ResetTimer()

				for b.Loop() {
					ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

					// Create metrics source
					source := make(chan analyzers.MetricsEvent, numEvents)
					broadcaster := analyzers.NewMetricsBroadcaster(source)

					// Start broadcaster
					go broadcaster.Serve(ctx)

					// Create subscribers
					subscribers := make([]<-chan analyzers.MetricsEvent, numSubscribers)
					for j := 0; j < numSubscribers; j++ {
						subscribers[j] = broadcaster.Subscribe()
					}

					// Send events
					go func() {
						defer close(source)
						for k := range numEvents {
							event := analyzers.MetricsEvent{
								Type:      analyzers.MetricsRequestSubmitted,
								Timestamp: time.Now(),
								WorkerID:  1,
								Data:      fmt.Sprintf("event-%d", k),
							}
							select {
							case source <- event:
							case <-ctx.Done():
								return
							}
						}
					}()

					// Read from subscribers
					var wg sync.WaitGroup
					for j, sub := range subscribers {
						wg.Add(1)
						go func(subscriberIndex int, subscriber <-chan analyzers.MetricsEvent) {
							defer wg.Done()
							received := 0
							timeout := time.After(5 * time.Second)
							for received < numEvents {
								select {
								case <-subscriber:
									received++
								case <-timeout:
									return
								case <-ctx.Done():
									return
								}
							}
						}(j, sub)
					}

					wg.Wait()
					cancel()
				}

				b.StopTimer()
				runtime.GC()
				runtime.ReadMemStats(&memAfter)

				allocDiff := memAfter.TotalAlloc - memBefore.TotalAlloc
				b.ReportMetric(float64(allocDiff)/float64(b.N), "bytes/op")
				b.ReportMetric(float64(numEvents), "events/iteration")
				b.ReportMetric(float64(numSubscribers), "subscribers")
			})
		}
	}
}
