package octo

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"testing"
	"time"
)

// BenchmarkRouter_NoMiddleware measures the performance of the router without any middleware.
func BenchmarkRouter_NoMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Add multiple routes
	for i := 0; i < 100; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/route50", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_WithMiddleware measures the performance of the router with middleware.
func BenchmarkRouter_WithMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Global middleware that does minimal work
	router.Use(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			next(ctx)
		}
	})

	// Add multiple routes
	for i := 0; i < 100; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/route50", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_HighConcurrency measures the router's performance under high concurrency.
func BenchmarkRouter_HighConcurrency(b *testing.B) {
	router := NewRouter[CustomData]()

	// Simulate a handler with some processing
	router.GET("/test", func(ctx *Ctx[CustomData]) {
		// Simulate processing delay
		time.Sleep(100 * time.Microsecond)
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Apply realistic middleware
	router.Use(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			// Simulate middleware overhead
			time.Sleep(50 * time.Microsecond)
			next(ctx)
		}
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/test", nil)

	b.ResetTimer()
	startTime := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})

	elapsed := time.Since(startTime)
	totalRequests := float64(b.N)
	throughput := totalRequests / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_Throughput_NoMiddleware measures throughput without middleware.
func BenchmarkRouter_Throughput_NoMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Add multiple routes
	for i := 0; i < 100; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/route50", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_Throughput_WithMiddleware measures throughput with middleware.
func BenchmarkRouter_Throughput_WithMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Global middleware that does minimal work
	router.Use(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			next(ctx)
		}
	})

	// Add multiple routes
	for i := 0; i < 100; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/route50", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

var largeResponse []byte

// BenchmarkRouter_LargeResponse measures performance when serving a large response.
func BenchmarkRouter_LargeResponse(b *testing.B) {
	if len(largeResponse) == 0 {
		largeResponse = make([]byte, 10*1024*1024) // Allocate once during package initialization
	}

	router := NewRouter[CustomData]()

	router.GET("/large", func(ctx *Ctx[CustomData]) {
		reader := bytes.NewReader(largeResponse)
		io.Copy(ctx.ResponseWriter, reader)
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/large", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body) // Read the response to simulate client behavior
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_DifferentMethods measures performance for different HTTP methods.
func BenchmarkRouter_DifferentMethods(b *testing.B) {
	router := NewRouter[CustomData]()

	// Handlers for different methods
	router.GET("/method", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("GET"))
	})
	router.POST("/method", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("POST"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}

	// Test GET method
	b.Run("GET", func(b *testing.B) {
		req, _ := http.NewRequest("GET", server.URL+"/method", nil)
		b.ResetTimer()
		startTime := time.Now()
		for i := 0; i < b.N; i++ {
			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		elapsed := time.Since(startTime)
		throughput := float64(b.N) / elapsed.Seconds()
		b.ReportMetric(throughput, "req/s")
	})

	// Test POST method
	b.Run("POST", func(b *testing.B) {
		req, _ := http.NewRequest("POST", server.URL+"/method", nil)
		b.ResetTimer()
		startTime := time.Now()
		for i := 0; i < b.N; i++ {
			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		elapsed := time.Since(startTime)
		throughput := float64(b.N) / elapsed.Seconds()
		b.ReportMetric(throughput, "req/s")
	})
}

// BenchmarkRouter_GCImpact measures the impact of garbage collection on performance.
func BenchmarkRouter_GCImpact(b *testing.B) {
	debug.SetGCPercent(100) // Adjust GC percentage as needed

	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/gc", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/gc", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_LargeNumberOfRoutes measures performance with a large number of routes.
func BenchmarkRouter_LargeNumberOfRoutes(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()

	numRoutes := 10000
	for i := 0; i < numRoutes; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/route5000", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// latencyMiddleware simulates network latency.
func latencyMiddleware(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
	return func(ctx *Ctx[CustomData]) {
		time.Sleep(10 * time.Millisecond) // Simulate network latency
		next(ctx)
	}
}

// BenchmarkRouter_WithNetworkLatency measures performance with simulated network latency.
func BenchmarkRouter_WithNetworkLatency(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.Use(latencyMiddleware)

	router.GET("/latency", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}

	b.ResetTimer()
	startTime := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		req, _ := http.NewRequest("GET", server.URL+"/latency", nil)
		for pb.Next() {
			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})

	elapsed := time.Since(startTime)
	totalRequests := float64(b.N)
	throughput := totalRequests / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_JSONResponse measures performance when sending JSON responses.
func BenchmarkRouter_JSONResponse(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	data := map[string]string{"message": "OK"}

	router.GET("/json", func(ctx *Ctx[CustomData]) {
		ctx.SendJSON(http.StatusOK, data)
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/json", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_Profiled measures performance while profiling.
func BenchmarkRouter_Profiled(b *testing.B) {
	router := NewRouter[CustomData]()
	router.GET("/profiled", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/profiled", nil)

	// Start profiling
	f, err := os.Create("cpu.prof")
	if err != nil {
		b.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_HeavyMiddleware measures performance with middleware that does significant work.
func BenchmarkRouter_HeavyMiddleware(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()

	// Middleware that does significant work
	router.Use(func(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
		return func(ctx *Ctx[CustomData]) {
			// Simulate heavy computation
			time.Sleep(1 * time.Millisecond)
			next(ctx)
		}
	})

	router.GET("/heavy", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/heavy", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_NotFound measures performance when handling 404 Not Found errors.
func BenchmarkRouter_NotFound(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	// No routes added

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/nonexistent", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_WithQueryParameters measures performance when handling query parameters.
func BenchmarkRouter_WithQueryParameters(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/search", func(ctx *Ctx[CustomData]) {
		query := ctx.QueryValue("q")
		ctx.ResponseWriter.Write([]byte("Query: " + query))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/search?q=test", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_DeepRoute measures performance with deeply nested routes.
func BenchmarkRouter_DeepRoute(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/level1/level2/level3/level4/level5", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Deep Route"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/level1/level2/level3/level4/level5", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_RouteConflicts measures performance when routes have potential conflicts.
func BenchmarkRouter_RouteConflicts(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/public/MessageExport/uuid/:uuid", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("UUID Route"))
	})
	router.GET("/public/MessageExport/order/:order_by", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Order By Route"))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	reqUUID, _ := http.NewRequest("GET", server.URL+"/public/MessageExport/uuid/123e4567-e89b-12d3-a456-426614174000", nil)
	reqOrder, _ := http.NewRequest("GET", server.URL+"/public/MessageExport/order/asc", nil)

	b.Run("UUID", func(b *testing.B) {
		b.ResetTimer()
		startTime := time.Now()
		for i := 0; i < b.N; i++ {
			resp, err := client.Do(reqUUID)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		elapsed := time.Since(startTime)
		throughput := float64(b.N) / elapsed.Seconds()
		b.ReportMetric(throughput, "req/s")
	})

	b.Run("OrderBy", func(b *testing.B) {
		b.ResetTimer()
		startTime := time.Now()
		for i := 0; i < b.N; i++ {
			resp, err := client.Do(reqOrder)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		elapsed := time.Since(startTime)
		throughput := float64(b.N) / elapsed.Seconds()
		b.ReportMetric(throughput, "req/s")
	})
}

// BenchmarkRouter_WildcardRoute measures performance when using wildcard routes.
func BenchmarkRouter_WildcardRoute(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/files/*filepath", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Filepath: " + ctx.Params["filepath"]))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/files/test.txt", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// BenchmarkRouter_ParameterizedRoute measures performance when using parameterized routes.
func BenchmarkRouter_ParameterizedRoute(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/user/:id", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("User ID: " + ctx.Params["id"]))
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/user/12345", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// Additional benchmark: BenchmarkRouter_StaticFileServing
// Measures performance when serving static files.
func BenchmarkRouter_StaticFileServing(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()

	// Simulate a static file handler
	router.GET("/files/*filepath", func(ctx *Ctx[CustomData]) {
		// Simulate reading a file (without actual disk I/O)
		data := []byte("File content")
		ctx.ResponseWriter.Write(data)
	})

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/files/test.txt", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.ReadAll(resp.Body)
		if err != nil {
			b.Fatal(err)
		}
		err = resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

// Additional benchmark: BenchmarkRouter_FileServerIntegration
// Measures performance when integrating with http.FileServer.
func BenchmarkRouter_FileServerIntegration(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()

	fs := http.FileServer(http.Dir(".")) // Adjust the directory as needed
	router.GET("/files/*filepath", func(ctx *Ctx[CustomData]) {
		ctx.Request.URL.Path = ctx.Params["filepath"]
		fs.ServeHTTP(ctx.ResponseWriter, ctx.Request)
	})

	// Create a temporary file to serve
	fileContent := []byte("Sample file content")
	tmpFile, err := os.CreateTemp("", "testfile*.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.Write(fileContent)
	if err != nil {
		b.Fatal(err)
	}
	err = tmpFile.Close()
	if err != nil {
		b.Fatal(err)
	}

	// Use an actual HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/files/"+tmpFile.Name(), nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.ReadAll(resp.Body)
		if err != nil {
			b.Fatal(err)
		}
		err = resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "req/s")
}

func BenchmarkRouter_LatencyMetrics(b *testing.B) {
	router := NewRouter[CustomData]()

	router.GET("/test", func(ctx *Ctx[CustomData]) {
		// Simulate processing delay
		time.Sleep(100 * time.Microsecond)
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/test", nil)

	var totalDuration time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		duration := time.Since(start)
		totalDuration += duration
	}

	avgLatency := totalDuration / time.Duration(b.N)
	b.ReportMetric(float64(avgLatency.Microseconds()), "avg_latency_us")
}

func BenchmarkRouter_MemoryProfiled(b *testing.B) {
	router := NewRouter[CustomData]()
	router.GET("/memory", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", server.URL+"/memory", nil)

	f, err := os.Create("mem.prof")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
