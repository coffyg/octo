package octo

import (
	"net/http/httptest"
	"testing"
)

func BenchmarkRouter(b *testing.B) {
	router := NewRouter[any]()
	
	// Static routes
	router.GET("/", func(ctx *Ctx[any]) {
		ctx.ResponseWriter.Write([]byte("Home"))
	})
	router.GET("/about", func(ctx *Ctx[any]) {
		ctx.ResponseWriter.Write([]byte("About"))
	})
	router.GET("/contact", func(ctx *Ctx[any]) {
		ctx.ResponseWriter.Write([]byte("Contact"))
	})
	
	// Parameter routes
	router.GET("/user/:id", func(ctx *Ctx[any]) {
		id := ctx.Params["id"]
		ctx.ResponseWriter.Write([]byte("User: " + id))
	})
	router.GET("/post/:id/comment/:cid", func(ctx *Ctx[any]) {
		pid := ctx.Params["id"]
		cid := ctx.Params["cid"]
		ctx.ResponseWriter.Write([]byte("Post: " + pid + ", Comment: " + cid))
	})
	
	// Wildcard route
	router.GET("/files/*path", func(ctx *Ctx[any]) {
		path := ctx.Params["path"]
		ctx.ResponseWriter.Write([]byte("File: " + path))
	})
	
	benchmarks := []struct {
		name string
		path string
	}{
		{"StaticRoot", "/"},
		{"StaticPath", "/about"},
		{"ParameterSingle", "/user/123"},
		{"ParameterMultiple", "/post/456/comment/789"},
		{"Wildcard", "/files/images/logo.png"},
		{"LongPath", "/files/deep/nested/directory/structure/file.txt"},
	}
	
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			req := httptest.NewRequest("GET", bm.path, nil)
			
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
		})
	}
}

func BenchmarkStaticRoutes(b *testing.B) {
	router := NewRouter[any]()
	
	// Add many static routes to test lookup performance
	paths := []string{
		"/api/v1/users",
		"/api/v1/posts",
		"/api/v1/comments",
		"/api/v2/users",
		"/api/v2/posts",
		"/api/v2/comments",
		"/admin/dashboard",
		"/admin/users",
		"/admin/settings",
		"/blog/posts",
		"/blog/categories",
		"/blog/tags",
	}
	
	handler := func(ctx *Ctx[any]) {
		ctx.ResponseWriter.Write([]byte("OK"))
	}
	
	for _, path := range paths {
		router.GET(path, handler)
	}
	
	// Benchmark different paths
	benchmarks := []struct {
		name string
		path string
	}{
		{"FirstRoute", "/api/v1/users"},
		{"MiddleRoute", "/admin/users"},
		{"LastRoute", "/blog/tags"},
	}
	
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			req := httptest.NewRequest("GET", bm.path, nil)
			
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
		})
	}
}

func BenchmarkMiddleware(b *testing.B) {
	router := NewRouter[any]()
	
	// Add some middleware
	router.Use(func(next HandlerFunc[any]) HandlerFunc[any] {
		return func(ctx *Ctx[any]) {
			ctx.SetHeader("X-Middleware-1", "true")
			next(ctx)
		}
	})
	
	router.Use(func(next HandlerFunc[any]) HandlerFunc[any] {
		return func(ctx *Ctx[any]) {
			ctx.SetHeader("X-Middleware-2", "true")
			next(ctx)
		}
	})
	
	router.GET("/test", func(ctx *Ctx[any]) {
		ctx.ResponseWriter.Write([]byte("Test"))
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkQueryParameters(b *testing.B) {
	router := NewRouter[any]()
	
	router.GET("/search", func(ctx *Ctx[any]) {
		q := ctx.QueryParam("q")
		page := ctx.QueryParam("page")
		limit := ctx.QueryParam("limit")
		ctx.ResponseWriter.Write([]byte("Search: " + q + " " + page + " " + limit))
	})
	
	req := httptest.NewRequest("GET", "/search?q=golang&page=2&limit=50", nil)
	
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}