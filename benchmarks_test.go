package octo

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Benchmark for routing without middleware
func BenchmarkRouter_NoMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Add multiple routes
	for i := 0; i < 100; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	req := httptest.NewRequest("GET", "/route50", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.ReadAll(resp.Body)
	}
}

// Benchmark for routing with middleware
func BenchmarkRouter_WithMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Global middleware
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

	req := httptest.NewRequest("GET", "/route50", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.ReadAll(resp.Body)
	}
}

// Benchmark for high concurrency
func BenchmarkRouter_HighConcurrency(b *testing.B) {
	router := NewRouter[CustomData]()

	router.GET("/test", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	b.SetParallelism(100) // Adjust the level of concurrency

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/test", nil)
		for pb.Next() {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			resp := w.Result()
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

// Benchmark for measuring throughput and latency without middleware
func BenchmarkRouter_Throughput_NoMiddleware(b *testing.B) {
	router := NewRouter[CustomData]()

	// Add multiple routes
	for i := 0; i < 100; i++ {
		path := "/route" + strconv.Itoa(i)
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	req := httptest.NewRequest("GET", "/route50", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()

	b.ReportMetric(throughput, "req/s")
}

// Benchmark for measuring throughput and latency with middleware
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

	req := httptest.NewRequest("GET", "/route50", nil)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	elapsed := time.Since(startTime)
	throughput := float64(b.N) / elapsed.Seconds()

	b.ReportMetric(throughput, "req/s")
}

func BenchmarkRouter_LargeResponse(b *testing.B) {
	router := NewRouter[CustomData]()

	// Generate a large response body
	largeResponse := make([]byte, 10*1024*1024) // 10 MB

	router.GET("/large", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write(largeResponse)
	})

	req := httptest.NewRequest("GET", "/large", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func BenchmarkRouter_DifferentMethods(b *testing.B) {
	router := NewRouter[CustomData]()

	// Handlers for different methods
	router.GET("/method", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("GET"))
	})
	router.POST("/method", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("POST"))
	})

	// Test GET method
	b.Run("GET", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/method", nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			resp := w.Result()
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})

	// Test POST method
	b.Run("POST", func(b *testing.B) {
		req := httptest.NewRequest("POST", "/method", nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			resp := w.Result()
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}
func BenchmarkRouter_GCImpact(b *testing.B) {
	debug.SetGCPercent(100) // Adjust as needed

	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/gc", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/gc", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
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

	req := httptest.NewRequest("GET", "/route5000", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func latencyMiddleware(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
	return func(ctx *Ctx[CustomData]) {
		time.Sleep(50 * time.Millisecond) // Simulate network latency
		next(ctx)
	}
}

func BenchmarkRouter_WithNetworkLatency(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.Use(latencyMiddleware)

	router.GET("/latency", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/latency", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func gzipMiddleware(next HandlerFunc[CustomData]) HandlerFunc[CustomData] {
	return func(ctx *Ctx[CustomData]) {
		if !strings.Contains(ctx.Request.Header.Get("Accept-Encoding"), "gzip") {
			next(ctx)
			return
		}
		gz := gzip.NewWriter(ctx.ResponseWriter)
		defer gz.Close()
		ctx.ResponseWriter.Header().Set("Content-Encoding", "gzip")
		ctx.ResponseWriter = &gzipResponseWriter{Writer: gz, ResponseWriter: ctx.ResponseWriter}
		next(ctx)
	}
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func BenchmarkRouter_WithGzipMiddleware(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.Use(gzipMiddleware)

	largeResponse := make([]byte, 10*1024*1024) // 10 MB
	for i := range largeResponse {
		largeResponse[i] = 'a'
	}

	router.GET("/gzip", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write(largeResponse)
	})

	req := httptest.NewRequest("GET", "/gzip", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func BenchmarkRouter_JSONResponse(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	data := map[string]string{"message": "OK"}

	router.GET("/json", func(ctx *Ctx[CustomData]) {
		ctx.SendJSON(http.StatusOK, data)
	})

	req := httptest.NewRequest("GET", "/json", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func BenchmarkRouter_Profiled(b *testing.B) {
	router := NewRouter[CustomData]()
	router.GET("/profiled", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/profiled", nil)

	// Start profiling
	f, err := os.Create("cpu.prof")
	if err != nil {
		b.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
func BenchmarkRouter_HighConcurrencyMultipleRoutes(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	paths := []string{"/route1", "/route2", "/route3", "/route4", "/route5"}

	for _, path := range paths {
		router.GET(path, func(ctx *Ctx[CustomData]) {
			ctx.ResponseWriter.Write([]byte("OK"))
		})
	}

	b.SetParallelism(100)

	b.RunParallel(func(pb *testing.PB) {
		reqs := []*http.Request{
			httptest.NewRequest("GET", "/route1", nil),
			httptest.NewRequest("GET", "/route2", nil),
			httptest.NewRequest("GET", "/route3", nil),
			httptest.NewRequest("GET", "/route4", nil),
			httptest.NewRequest("GET", "/route5", nil),
		}
		idx := 0
		for pb.Next() {
			req := reqs[idx%len(reqs)]
			idx++
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			resp := w.Result()
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

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

	req := httptest.NewRequest("GET", "/heavy", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkRouter_NotFound(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	// No routes added

	req := httptest.NewRequest("GET", "/nonexistent", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkRouter_WithQueryParameters(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/search", func(ctx *Ctx[CustomData]) {
		query := ctx.QueryValue("q")
		ctx.ResponseWriter.Write([]byte("Query: " + query))
	})

	req := httptest.NewRequest("GET", "/search?q=test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkRouter_DeepRoute(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/level1/level2/level3/level4/level5", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Deep Route"))
	})

	req := httptest.NewRequest("GET", "/level1/level2/level3/level4/level5", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkRouter_RouteConflicts(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/public/MessageExport/uuid/:uuid", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("UUID Route"))
	})
	router.GET("/public/MessageExport/order/:order_by", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Order By Route"))
	})

	reqUUID := httptest.NewRequest("GET", "/public/MessageExport/uuid/123e4567-e89b-12d3-a456-426614174000", nil)
	reqOrder := httptest.NewRequest("GET", "/public/MessageExport/order/asc", nil)

	b.Run("UUID", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, reqUUID)
			resp := w.Result()
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})

	b.Run("OrderBy", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, reqOrder)
			resp := w.Result()
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

func BenchmarkRouter_WildcardRoute(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/static/*filepath", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("Filepath: " + ctx.Params["filepath"]))
	})

	req := httptest.NewRequest("GET", "/static/css/style.css", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkRouter_ParameterizedRoute(b *testing.B) {
	b.ReportAllocs()
	router := NewRouter[CustomData]()
	router.GET("/user/:id", func(ctx *Ctx[CustomData]) {
		ctx.ResponseWriter.Write([]byte("User ID: " + ctx.Params["id"]))
	})

	req := httptest.NewRequest("GET", "/user/12345", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		resp := w.Result()
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
