package octo

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

// BenchmarkMiddlewareChainApplication tests the performance of middleware application
func BenchmarkMiddlewareChainApplication(b *testing.B) {
	type testData struct{}
	
	// Test case for different numbers of middleware
	testCases := []struct {
		name            string
		middlewareCount int
	}{
		{"NoMiddleware", 0},
		{"OneMiddleware", 1},
		{"FiveMiddleware", 5},
		{"TenMiddleware", 10},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Create a basic handler
			handler := func(ctx *Ctx[testData]) {
				ctx.ResponseWriter.Write([]byte("OK"))
			}

			// Create dummy middleware
			var middlewareChain []MiddlewareFunc[testData]
			for i := 0; i < tc.middlewareCount; i++ {
				middlewareChain = append(middlewareChain, func(next HandlerFunc[testData]) HandlerFunc[testData] {
					return func(ctx *Ctx[testData]) {
						// Minimal work in middleware
						next(ctx)
					}
				})
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Apply middleware and execute the handler
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/test", nil)
				ctx := &Ctx[testData]{
					ResponseWriter: NewResponseWriterWrapper(w),
					Request:        req,
					Params:         make(map[string]string),
				}

				finalHandler := applyMiddleware[testData](handler, middlewareChain)
				finalHandler(ctx)
			}
		})
	}
}

// BenchmarkMiddlewareImplementations compares different middleware implementation approaches
func BenchmarkMiddlewareImplementations(b *testing.B) {
	type testData struct{}
	
	// Test different middleware application implementations
	testCases := []struct {
		name string
		fn   func(handler HandlerFunc[testData], middleware []MiddlewareFunc[testData]) HandlerFunc[testData]
	}{
		{
			name: "Current",
			fn:   applyMiddleware[testData],
		},
		{
			name: "Alternative",
			fn: func(handler HandlerFunc[testData], middleware []MiddlewareFunc[testData]) HandlerFunc[testData] {
				// Alternative implementation for comparison
				if len(middleware) == 0 {
					return handler
				}

				// Apply middleware in reverse order (last middleware executes first)
				result := handler
				for i := len(middleware) - 1; i >= 0; i-- {
					mw := middleware[i]
					result = mw(result)
				}

				return result
			},
		},
		{
			name: "WithFastPaths",
			fn: func(handler HandlerFunc[testData], middleware []MiddlewareFunc[testData]) HandlerFunc[testData] {
				// Fast paths for common cases
				switch len(middleware) {
				case 0:
					return handler
				case 1:
					return middleware[0](handler)
				case 2:
					return middleware[1](middleware[0](handler))
				case 3:
					return middleware[2](middleware[1](middleware[0](handler)))
				default:
					// For more middleware, use the general approach
					result := handler
					for i := len(middleware) - 1; i >= 0; i-- {
						mw := middleware[i]
						result = mw(result)
					}
					return result
				}
			},
		},
	}

	// Test with different middleware counts
	middlewareCounts := []int{0, 1, 3, 5, 10}

	for _, impl := range testCases {
		for _, count := range middlewareCounts {
			name := fmt.Sprintf("%s_%dMW", impl.name, count)
			b.Run(name, func(b *testing.B) {
				// Create a basic handler
				handler := func(ctx *Ctx[testData]) {
					ctx.ResponseWriter.Write([]byte("OK"))
				}

				// Create dummy middleware
				var middlewareChain []MiddlewareFunc[testData]
				for i := 0; i < count; i++ {
					middlewareChain = append(middlewareChain, func(next HandlerFunc[testData]) HandlerFunc[testData] {
						return func(ctx *Ctx[testData]) {
							// Minimal work in middleware
							next(ctx)
						}
					})
				}

				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					// Apply middleware using the implementation being tested
					finalHandler := impl.fn(handler, middlewareChain)

					// Execute handler with a test context
					w := httptest.NewRecorder()
					req := httptest.NewRequest("GET", "/test", nil)
					ctx := &Ctx[testData]{
						ResponseWriter: NewResponseWriterWrapper(w),
						Request:        req,
						Params:         make(map[string]string),
					}
					finalHandler(ctx)
				}
			})
		}
	}
}