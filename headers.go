package octo

// Pre-compiled common header names as byte slices for zero-allocation header operations
var (
	// Request headers (commonly read)
	headerContentTypeBytes       = []byte("Content-Type")
	headerXForwardedForBytes     = []byte("X-Forwarded-For")
	headerXRealIPBytes           = []byte("X-Real-IP")
	headerIfNoneMatchBytes       = []byte("If-None-Match")
	headerIfModifiedSinceBytes   = []byte("If-Modified-Since")
	headerUserAgentBytes         = []byte("User-Agent")
	headerAcceptBytes            = []byte("Accept")
	headerAcceptLanguageBytes    = []byte("Accept-Language")
	headerAcceptEncodingBytes    = []byte("Accept-Encoding")
	headerXRequestIDBytes        = []byte("X-Request-ID")
	headerXCorrelationIDBytes    = []byte("X-Correlation-ID")

	// Response headers (commonly written)
	headerContentLengthBytes     = []byte("Content-Length")
	headerCacheControlBytes      = []byte("Cache-Control")
	headerETagBytes              = []byte("ETag")
	headerLastModifiedBytes      = []byte("Last-Modified")
	headerAllowBytes             = []byte("Allow")
	headerXContentTypeOptionsBytes = []byte("X-Content-Type-Options")
	headerXFrameOptionsBytes     = []byte("X-Frame-Options")
	headerXXSSProtectionBytes    = []byte("X-XSS-Protection")

	// Common content types
	contentTypeJSONBytes         = []byte("application/json")
	contentTypeXMLBytes          = []byte("application/xml")
	contentTypeHTMLBytes         = []byte("text/html; charset=utf-8")
	contentTypePlainBytes        = []byte("text/plain; charset=utf-8")
	contentTypeOctetStreamBytes  = []byte("application/octet-stream")

	// Common cache control values
	cacheControlNoStoreBytes     = []byte("no-store")
	cacheControlPublicBytes      = []byte("public, max-age=")

	// Security header values
	nosniffBytes                 = []byte("nosniff")
	denyBytes                    = []byte("DENY")
	xssProtectionBytes           = []byte("1; mode=block")
)

// HeaderName constants for type-safe header operations
const (
	HeaderContentType         = "Content-Type"
	HeaderContentLength       = "Content-Length"
	HeaderCacheControl        = "Cache-Control"
	HeaderETag                = "ETag"
	HeaderLastModified        = "Last-Modified"
	HeaderIfNoneMatch         = "If-None-Match"
	HeaderIfModifiedSince     = "If-Modified-Since"
	HeaderXForwardedFor       = "X-Forwarded-For"
	HeaderXRealIP             = "X-Real-IP"
	HeaderAllow               = "Allow"
	HeaderUserAgent           = "User-Agent"
	HeaderAccept              = "Accept"
	HeaderAcceptLanguage      = "Accept-Language"
	HeaderAcceptEncoding      = "Accept-Encoding"
	HeaderXRequestID          = "X-Request-ID"
	HeaderXCorrelationID      = "X-Correlation-ID"
	HeaderXContentTypeOptions = "X-Content-Type-Options"
	HeaderXFrameOptions       = "X-Frame-Options"
	HeaderXXSSProtection      = "X-XSS-Protection"
)

// ContentType constants
const (
	ContentTypeJSON        = "application/json"
	ContentTypeXML         = "application/xml"
	ContentTypeHTML        = "text/html; charset=utf-8"
	ContentTypePlain       = "text/plain; charset=utf-8"
	ContentTypeOctetStream = "application/octet-stream"
)