package octo

import (
    "context"
)

// Context key for storing request information
type RequestErrorKey struct{}

// Captures essential request details for error context without sensitive data
type RequestErrorInfo struct {
    Method  string
    Path    string
    IP      string
    Headers map[string]string
}

// Captures request context for enhanced error debugging
func ErrorContextMiddleware[V any]() MiddlewareFunc[V] {
    return func(next HandlerFunc[V]) HandlerFunc[V] {
        return func(ctx *Ctx[V]) {
            info := &RequestErrorInfo{
                Method:  ctx.Request.Method,
                Path:    ctx.Request.URL.Path,
                IP:      ctx.ClientIP(),
                Headers: make(map[string]string),
            }
            
            // Only collect non-sensitive headers
            safeHeaders := []string{
                HeaderUserAgent,
                HeaderAccept,
                HeaderAcceptLanguage,
                HeaderAcceptEncoding,
                HeaderContentType,
                HeaderContentLength,
                HeaderXRequestID,
                HeaderXCorrelationID,
            }
            
            for _, header := range safeHeaders {
                if value := ctx.GetHeader(header); value != "" {
                    info.Headers[header] = value
                }
            }
            
            ctx.Request = ctx.Request.WithContext(
                context.WithValue(ctx.Request.Context(), RequestErrorKey{}, info),
            )
            
            next(ctx)
        }
    }
}

// Enhances error logs with HTTP request context
func EnhanceLogErrorsMiddleware[V any]() MiddlewareFunc[V] {
    return func(next HandlerFunc[V]) HandlerFunc[V] {
        return func(ctx *Ctx[V]) {
            next(ctx)
            
            // Only process for completed requests with error status codes
            if ctx.IsDone() && ctx.ResponseWriter.Status >= 400 {
                info, ok := ctx.Request.Context().Value(RequestErrorKey{}).(*RequestErrorInfo)
                if ok && logger != nil {
                    if EnableLoggerCheck && logger == nil {
                        return
                    }
                    
                    event := logger.Error().
                        Int("status_code", ctx.ResponseWriter.Status).
                        Str("path", info.Path).
                        Str("method", info.Method).
                        Str("ip", info.IP)
                    
                    for k, v := range info.Headers {
                        event = event.Str("header_"+k, v)
                    }
                    
                    event.Msg("[octo] Request resulted in error")
                }
            }
        }
    }
}