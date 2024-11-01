package octo

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
)

type Ctx[V any] struct {
	ResponseWriter http.ResponseWriter  `json:"-"`
	Request        *http.Request        `json:"-"`
	Params         map[string]string    `json:"Params"`
	Query          map[string]*[]string `json:"Query"`
	StartTime      int64                `json:"StartTime"`
	UUID           string               `json:"UUID"`
	Body           []byte               `json:"-"`
	Headers        http.Header          `json:"-"`
	Custom         V                    // Generic Custom Field
}

func (c *Ctx[V]) SetHeader(key, value string) {
	c.ResponseWriter.Header().Set(key, value)
}

func (c *Ctx[V]) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

func (c *Ctx[V]) DelHeader(key string) {
	c.ResponseWriter.Header().Del(key)
}

func (c *Ctx[V]) GetParam(key string) string {
	return c.Params[key]
}

func (c *Ctx[V]) SetParam(key, value string) {
	c.Params[key] = value
}

func (c *Ctx[V]) SetStatus(code int) {
	c.ResponseWriter.WriteHeader(code)
}

func (c *Ctx[V]) SendJSON(statusCode int, v interface{}) {
	c.SetHeader("Content-Type", "application/json")
	response, err := sonic.Marshal(v)
	if err != nil {
		c.SetStatus(http.StatusInternalServerError)
		c.ResponseWriter.Write([]byte(`"error encoding response: ` + err.Error() + `"`))
		return
	}
	c.SetStatus(statusCode)
	c.ResponseWriter.Write(response)
}

func (c *Ctx[V]) QueryParam(key string) string {
	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c *Ctx[V]) DefaultQuery(key, defaultValue string) string {
	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}
	return defaultValue
}

func (c *Ctx[V]) QueryArray(key string) []string {
	return c.Request.URL.Query()[key]
}

func (c *Ctx[V]) QueryMap() map[string][]string {
	return c.Request.URL.Query()
}

func (c *Ctx[V]) Context() context.Context {
	return c.Request.Context()
}

func (c *Ctx[V]) Done() {
	c.Request.Context().Done()
}

// ClientIP returns the client's IP address, even if behind a proxy
func (c *Ctx[V]) ClientIP() string {
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		if idx := strings.IndexByte(ip, ','); idx != -1 {
			ip = ip[:idx]
		}
		return strings.TrimSpace(ip)
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// Cookie retrieves the value of the named cookie from the request.
// It returns an error if the cookie is not present.
func (c *Ctx[V]) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie adds a Set-Cookie header to the response.
func (c *Ctx[V]) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	http.SetCookie(c.ResponseWriter, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}
