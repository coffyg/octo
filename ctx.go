package octo

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/go-playground/form/v4"
)

var formDecoder = form.NewDecoder()

type Ctx[V any] struct {
	ResponseWriter http.ResponseWriter `json:"-"`
	Request        *http.Request       `json:"-"`
	Params         map[string]string   `json:"Params"`
	Query          map[string][]string `json:"Query"`
	StartTime      int64               `json:"StartTime"`
	UUID           string              `json:"UUID"`
	Body           []byte              `json:"-"`
	Headers        http.Header         `json:"-"`
	Custom         V                   // Generic Custom Field
	done           bool
	hasReadBody    bool
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
		c.ResponseWriter.Write([]byte("error encoding response: " + err.Error()))
		return
	}
	c.SetStatus(statusCode)
	c.ResponseWriter.Write(response)
}

func (c *Ctx[V]) QueryParam(key string) string {
	// check if it's in params
	if value, ok := c.Params[key]; ok {
		return value
	}

	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c *Ctx[V]) DefaultQueryParam(key, defaultValue string) string {
	// check if it's in params
	if value, ok := c.Params[key]; ok {
		return value
	}

	values := c.Request.URL.Query()[key]
	if len(values) > 0 {
		return values[0]
	}

	return defaultValue
}

func (c *Ctx[V]) QueryValue(key string) string {
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
	c.done = true
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
	if maxAge <= 0 {
		maxAge = -1
	}
	if path == "" {
		path = "/"
	}
	if domain == "" {
		domain = c.Request.URL.Hostname()
	}
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

// BindJSON binds the request body into an interface.
func (c *Ctx[V]) ShouldBindJSON(obj interface{}) error {
	c.NeedBody()
	if len(c.Body) == 0 {
		return errors.New("request body is empty")
	}
	return sonic.Unmarshal(c.Body, obj)
}

// ShouldBindXML binds the XML request body into the provided object.
func (c *Ctx[V]) ShouldBindXML(obj interface{}) error {
	c.NeedBody()
	if len(c.Body) == 0 {
		return errors.New("request body is empty")
	}
	return xml.Unmarshal(c.Body, obj)
}

// ShouldBindForm binds form data into the provided object.
func (c *Ctx[V]) ShouldBindForm(obj interface{}) error {
	c.NeedBody()
	// Reset Request.Body
	c.Request.Body = io.NopCloser(bytes.NewBuffer(c.Body))

	if err := c.Request.ParseForm(); err != nil {
		return err
	}
	values := c.Request.PostForm
	return mapForm(obj, values)
}

// ShouldBindMultipartForm binds multipart form data into the provided object.
func (c *Ctx[V]) ShouldBindMultipartForm(obj interface{}) error {
	c.NeedBody()
	// Reset Request.Body
	c.Request.Body = io.NopCloser(bytes.NewBuffer(c.Body))

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return err
	}
	values := c.Request.MultipartForm.Value
	return mapForm(obj, values)
}

// mapForm maps form values into the provided struct.
func mapForm(ptr interface{}, formData url.Values) error {
	return formDecoder.Decode(ptr, formData)
}

// ShouldBind binds the request body into the provided object
// according to the Content-Type header.
func (c *Ctx[V]) ShouldBind(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	contentType, _, _ = mime.ParseMediaType(contentType)

	switch contentType {
	case "application/json":
		return c.ShouldBindJSON(obj)
	case "application/xml", "text/xml":
		return c.ShouldBindXML(obj)
	case "application/x-www-form-urlencoded":
		return c.ShouldBindForm(obj)
	case "multipart/form-data":
		return c.ShouldBindMultipartForm(obj)
	default:
		return errors.New("unsupported content type: " + contentType)
	}
}

func (c *Ctx[V]) NeedBody() error {
	if c.hasReadBody {
		return nil
	}
	var body []byte
	var err error
	body, err = io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Error().Err(err).Msg("[octo] failed to read request body")
		return err
	}

	c.Body = body
	c.hasReadBody = true
	return nil
}
