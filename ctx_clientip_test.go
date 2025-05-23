package octo

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		expected      string
	}{
		{
			name:       "Empty RemoteAddr",
			remoteAddr: "",
			expected:   "0.0.0.0",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "RemoteAddr with port",
			remoteAddr: "192.168.1.1:8080",
			expected:   "192.168.1.1",
		},
		{
			name:       "IPv6 without port",
			remoteAddr: "::1",
			expected:   "::1",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[::1]:8080",
			expected:   "::1",
		},
		{
			name:       "Malformed RemoteAddr",
			remoteAddr: "not-an-ip",
			expected:   "not-an-ip",
		},
		{
			name:          "X-Forwarded-For single IP",
			remoteAddr:    "192.168.1.1:8080",
			xForwardedFor: "10.0.0.1",
			expected:      "10.0.0.1",
		},
		{
			name:          "X-Forwarded-For multiple IPs",
			remoteAddr:    "192.168.1.1:8080",
			xForwardedFor: "10.0.0.1, 10.0.0.2, 10.0.0.3",
			expected:      "10.0.0.1",
		},
		{
			name:          "X-Forwarded-For with spaces",
			remoteAddr:    "192.168.1.1:8080",
			xForwardedFor: "  10.0.0.1  ",
			expected:      "10.0.0.1",
		},
		{
			name:          "X-Forwarded-For invalid IP",
			remoteAddr:    "192.168.1.1:8080",
			xForwardedFor: "not-an-ip",
			expected:      "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "192.168.1.1:8080",
			xRealIP:    "10.0.0.2",
			expected:   "10.0.0.2",
		},
		{
			name:       "X-Real-IP with spaces",
			remoteAddr: "192.168.1.1:8080",
			xRealIP:    "  10.0.0.2  ",
			expected:   "10.0.0.2",
		},
		{
			name:       "X-Real-IP invalid",
			remoteAddr: "192.168.1.1:8080",
			xRealIP:    "not-an-ip",
			expected:   "192.168.1.1",
		},
		{
			name:          "Priority test: X-Forwarded-For over X-Real-IP",
			remoteAddr:    "192.168.1.1:8080",
			xForwardedFor: "10.0.0.1",
			xRealIP:       "10.0.0.2",
			expected:      "10.0.0.1",
		},
		{
			name:       "IPv6 in brackets without port",
			remoteAddr: "[::1]",
			expected:   "::1", // We extract the IP from brackets
		},
		{
			name:       "Weird format with multiple colons",
			remoteAddr: "192.168.1.1:8080:extra",
			expected:   "192.168.1.1:8080:extra", // Return as-is for unparseable formats
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			w := httptest.NewRecorder()
			ctx := &Ctx[any]{
				Request:        req,
				ResponseWriter: NewResponseWriterWrapper(w),
			}

			got := ctx.ClientIP()
			if got != tt.expected {
				t.Errorf("ClientIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}