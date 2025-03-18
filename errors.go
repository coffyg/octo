package octo

import (
    "fmt"
    "net/http"
    "runtime"
    "strings"

    "github.com/pkg/errors"
    "github.com/rs/zerolog"
)

// Unique identifier for categorizing errors on both server and client sides
type ErrorCode string

const (
    // Common errors
    ErrUnknown         ErrorCode = "err_unknown_error"
    ErrInternal        ErrorCode = "err_internal_error"
    ErrInvalidRequest  ErrorCode = "err_invalid_request"
    ErrNotFound        ErrorCode = "err_not_found"
    ErrUnauthorized    ErrorCode = "err_unauthorized"
    ErrForbidden       ErrorCode = "err_forbidden"
    ErrTimeout         ErrorCode = "err_timeout"
    ErrTooManyRequests ErrorCode = "err_too_many_requests"
    
    // Database errors
    ErrDBError         ErrorCode = "err_db_error"
    ErrDBNotFound      ErrorCode = "err_db_not_found"
    ErrDBDuplicate     ErrorCode = "err_db_duplicate"
    
    // Validation errors
    ErrValidation      ErrorCode = "err_validation"
    ErrInvalidJSON     ErrorCode = "err_invalid_json"
    ErrInvalidForm     ErrorCode = "err_invalid_form"
    
    // Authentication errors
    ErrAuthFailed      ErrorCode = "err_auth_failed"
    ErrTokenExpired    ErrorCode = "err_token_expired"
    ErrTokenInvalid    ErrorCode = "err_token_invalid"
)

// Standardized error type with rich context for debugging and client communication
type OctoError struct {
    Original   error      // The underlying error being wrapped
    Code       ErrorCode  // Client-facing error code
    StatusCode int        // HTTP status code
    Message    string     // Human-readable error message
    
    // Debug information automatically captured
    file       string
    line       int
    function   string
}

// Maps error codes to HTTP status codes and default messages
type APIErrorDef struct {
    Message    string
    StatusCode int
}

var PredefinedErrors = map[ErrorCode]APIErrorDef{
    ErrUnknown:         {"Unknown error", http.StatusInternalServerError},
    ErrInternal:        {"Internal error", http.StatusInternalServerError},
    ErrDBError:         {"Database error", http.StatusInternalServerError},
    ErrInvalidRequest:  {"Invalid request", http.StatusBadRequest},
    ErrNotFound:        {"Not found", http.StatusNotFound},
    ErrUnauthorized:    {"Unauthorized", http.StatusUnauthorized},
    ErrForbidden:       {"Forbidden", http.StatusForbidden},
    ErrTimeout:         {"Request timeout", http.StatusRequestTimeout},
    ErrTooManyRequests: {"Too many requests", http.StatusTooManyRequests},
    ErrDBNotFound:      {"Resource not found", http.StatusNotFound},
    ErrDBDuplicate:     {"Resource already exists", http.StatusConflict},
    ErrValidation:      {"Validation error", http.StatusBadRequest},
    ErrInvalidJSON:     {"Invalid JSON", http.StatusBadRequest},
    ErrInvalidForm:     {"Invalid form data", http.StatusBadRequest},
    ErrAuthFailed:      {"Authentication failed", http.StatusUnauthorized},
    ErrTokenExpired:    {"Authentication token expired", http.StatusUnauthorized},
    ErrTokenInvalid:    {"Invalid authentication token", http.StatusUnauthorized},
}

func (e *OctoError) Error() string {
    base := fmt.Sprintf("[octo:%s] %s", e.Code, e.Message)
    if e.Original != nil {
        return fmt.Sprintf("%s: %v", base, e.Original)
    }
    return base
}

func (e *OctoError) Unwrap() error {
    return e.Original
}

func New(code ErrorCode, msg string) *OctoError {
    def, ok := PredefinedErrors[code]
    if !ok {
        def = PredefinedErrors[ErrUnknown]
    }
    
    if msg == "" {
        msg = def.Message
    }
    
    err := &OctoError{
        Code:       code,
        StatusCode: def.StatusCode,
        Message:    msg,
    }
    
    // Automatically capture caller information for debugging
    if pc, file, line, ok := runtime.Caller(1); ok {
        err.file = file
        err.line = line
        if fn := runtime.FuncForPC(pc); fn != nil {
            err.function = fn.Name()
        }
    }
    
    return err
}

func Newf(code ErrorCode, format string, args ...interface{}) *OctoError {
    return New(code, fmt.Sprintf(format, args...))
}

func Wrap(err error, code ErrorCode, msg string) *OctoError {
    if err == nil {
        return nil
    }
    
    // If already an OctoError, update its fields instead of creating new one
    if octoErr, ok := err.(*OctoError); ok {
        if code != "" {
            octoErr.Code = code
            
            if def, ok := PredefinedErrors[code]; ok {
                octoErr.StatusCode = def.StatusCode
            }
        }
        
        if msg != "" {
            octoErr.Message = msg
        }
        
        if pc, file, line, ok := runtime.Caller(1); ok {
            octoErr.file = file
            octoErr.line = line
            if fn := runtime.FuncForPC(pc); fn != nil {
                octoErr.function = fn.Name()
            }
        }
        
        return octoErr
    }
    
    // Create a new OctoError wrapping the original
    octoErr := New(code, msg)
    octoErr.Original = err
    
    if pc, file, line, ok := runtime.Caller(1); ok {
        octoErr.file = file
        octoErr.line = line
        if fn := runtime.FuncForPC(pc); fn != nil {
            octoErr.function = fn.Name()
        }
    }
    
    return octoErr
}

func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *OctoError {
    return Wrap(err, code, fmt.Sprintf(format, args...))
}

func Is(err error, code ErrorCode) bool {
    if err == nil {
        return false
    }
    
    var octoErr *OctoError
    if errors.As(err, &octoErr) {
        return octoErr.Code == code
    }
    
    return false
}

// Implements the safety rule of checking function preconditions and postconditions
func Assert(condition bool, code ErrorCode, message string) error {
    if !condition {
        return New(code, message)
    }
    return nil
}

func AssertWithError(condition bool, err error) error {
    if !condition {
        return err
    }
    return nil
}

// For critical assertions where failure indicates a programming error
func MustAssert(condition bool, message string) {
    if !condition {
        panic(New(ErrInternal, "Assertion failed: "+message))
    }
}

// Logs errors with appropriate context and stack trace
func LogError(logger *zerolog.Logger, err error) {
    if err == nil || logger == nil {
        return
    }
    
    event := logger.Error().Err(err)
    
    // Add caller information based on error type
    if _, ok := err.(*OctoError); !ok {
        if pc, file, line, ok := runtime.Caller(1); ok {
            shortFile := file
            if idx := strings.LastIndex(file, "/"); idx >= 0 {
                shortFile = file[idx+1:]
            }
            
            funcName := "unknown"
            if fn := runtime.FuncForPC(pc); fn != nil {
                funcName = fn.Name()
                if idx := strings.LastIndex(funcName, "."); idx >= 0 {
                    funcName = funcName[idx+1:]
                }
            }
            
            event = event.Str("file", shortFile).Int("line", line).Str("function", funcName)
        }
    } else {
        octoErr := err.(*OctoError)
        event = event.
            Str("error_code", string(octoErr.Code)).
            Int("status_code", octoErr.StatusCode).
            Str("file", octoErr.file).
            Int("line", octoErr.line).
            Str("function", octoErr.function)
    }
    
    event.Msg("[octo-error] Error occurred")
}

// Handles and logs recovered panics with full context
func LogPanic(logger *zerolog.Logger, recovered interface{}, stack []byte) {
    if logger == nil {
        return
    }
    
    var err error
    switch v := recovered.(type) {
    case error:
        err = v
    case string:
        err = errors.New(v)
    default:
        err = errors.Errorf("%v", recovered)
    }
    
    wrappedErr := Wrap(err, ErrInternal, "Panic recovered")
    
    stackStr := string(stack)
    logger.Error().
        Err(wrappedErr).
        Str("error_code", string(wrappedErr.Code)).
        Int("status_code", wrappedErr.StatusCode).
        Str("stack_trace", stackStr).
        Msg("[octo-panic] Panic recovered")
}