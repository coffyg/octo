package octo

import (
	"strings"
)

// ConnectionDetectionMiddleware detects the connection type early in the request pipeline
// This MUST be the first middleware in the chain to ensure connection type is available
// for panic recovery and other middleware
func ConnectionDetectionMiddleware[V any]() MiddlewareFunc[V] {
	return func(next HandlerFunc[V]) HandlerFunc[V] {
		return func(ctx *Ctx[V]) {
			// Detect connection type immediately
			if ctx.Request != nil {
				// Check for WebSocket upgrade
				if strings.EqualFold(ctx.Request.Header.Get("Connection"), "Upgrade") &&
				   strings.EqualFold(ctx.Request.Header.Get("Upgrade"), "websocket") {
					ctx.ConnectionType = ConnectionTypeWebSocket
				} else if strings.Contains(ctx.Request.Header.Get("Accept"), "text/event-stream") {
					// Check for SSE based on Accept header
					ctx.ConnectionType = ConnectionTypeSSE
				} else {
					ctx.ConnectionType = ConnectionTypeHTTP
				}
			}
			
			next(ctx)
		}
	}
}