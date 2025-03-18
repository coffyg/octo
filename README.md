# Octo Framework

A fast, type-safe HTTP router and middleware framework for Go with generic context support.

## Features

- Type-safe request context using generics
- Fast router with wildcard and parameter support
- Middleware system with proper context propagation
- Error handling with custom error types and stack traces
- Performance-optimized request handling

## Quick Start

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/coffyg/octo"
)

type CustomData struct {
    UserID   string
    IsAdmin  bool
}

func main() {
    // Create a new router with custom context data
    router := octo.NewRouter[CustomData]()

    // Add middleware
    router.Use(octo.LoggerMiddleware[CustomData]())
    router.Use(octo.RecoveryMiddleware[CustomData]())

    // Define routes
    router.GET("/users/:id", func(ctx *octo.Ctx[CustomData]) {
        id := ctx.Params["id"]
        ctx.Custom.UserID = id
        ctx.SendJSON(http.StatusOK, map[string]string{
            "id": id,
            "message": "User found",
        })
    })

    // Start server
    http.ListenAndServe(":8080", router)
}
```

## Development

### Testing

Run the tests:

```bash
go test -v ./...
```

Run tests with race detection:

```bash
go test -race ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem
```

### Code Style

This project follows the Coffyg Go style guide. To check code style:

```bash
# Install brewtest
go install github.com/coffyg/brewtest@latest

# Run style checks
brewtest --verbose .

# Check specific style aspects
brewtest --config .brewtest.json .
```

The project includes a `.brewtest.json` configuration file that defines the style rules.

### Style Guide

The coding style guidelines for this project are documented in [coding_style.md](coding_style.md).

## Middleware

The framework uses a chained middleware system that is both type-safe and highly performant:

```go
// Define custom middleware
func AuthMiddleware[V any]() octo.MiddlewareFunc[V] {
    return func(next octo.HandlerFunc[V]) octo.HandlerFunc[V] {
        return func(ctx *octo.Ctx[V]) {
            // Authentication logic
            token := ctx.GetHeader("Authorization")
            if token == "" {
                ctx.Send401()
                return
            }
            // Continue request handling
            next(ctx)
        }
    }
}

// Apply middleware to specific routes
router.GET("/admin/dashboard", AdminHandler).Use(AuthMiddleware[CustomData]())
```

## Performance

The Octo framework has been optimized for performance:

- Benchmark results show 18,000+ requests/second on commodity hardware
- Optimized middleware chain application
- Efficient route matching algorithm
- Minimal memory allocations per request

## License

MIT License
