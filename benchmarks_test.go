package octo

import (
	"io"
	"net/http/httptest"
	"strconv"
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
