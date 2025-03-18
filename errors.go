// Package octo provides a lightweight HTTP framework with router capabilities.
package octo

import (
    "fmt"
    "net/http"
    "runtime"
    "strings"

    "github.com/pkg/errors"
    "github.com/rs/zerolog"
)

// ErrorCode represents a unique error identifier used for client-side error handling.
type ErrorCode string

// HTTP status codes mapped to common error scenarios.
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

// OctoError represents a standardized application error with context.
type OctoError struct {
    // Original is the original error that was wrapped
    Original error
    
    // Code is the error code string for client-side error handling
    Code ErrorCode
    
    // StatusCode is the HTTP status code associated with this error
    StatusCode int
    
    // Message is a human-readable error message
    Message string
    
    // File and line information where the error occurred
    file string
    line int
    
    // Function name where the error occurred
    function string
}

// APIErrorDef maps error codes to status codes and default messages.
type APIErrorDef struct {
    Message    string
    StatusCode int
}

// PredefinedErrors maps error codes to their definitions.
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

// Error returns the error message with context information.
func (e *OctoError) Error() string {
    base := fmt.Sprintf("[octo:%s] %s", e.Code, e.Message)
    if e.Original != nil {
        return fmt.Sprintf("%s: %v", base, e.Original)
    }
    return base
}

// Unwrap implements the errors.Unwrap interface.
func (e *OctoError) Unwrap() error {
    return e.Original
}

// New creates a new OctoError with the given code and message.
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
    
    // Capture caller information
    if pc, file, line, ok := runtime.Caller(1); ok {
        err.file = file
        err.line = line
        if fn := runtime.FuncForPC(pc); fn != nil {
            err.function = fn.Name()
        }
    }
    
    return err
}

// Newf creates a new OctoError with formatted message.
func Newf(code ErrorCode, format string, args ...interface{}) *OctoError {
    return New(code, fmt.Sprintf(format, args...))
}

// Wrap wraps an existing error with additional context.
func Wrap(err error, code ErrorCode, msg string) *OctoError {
    if err == nil {
        return nil
    }
    
    // If already an OctoError, just update the fields
    if octoErr, ok := err.(*OctoError); ok {
        if code != "" {
            octoErr.Code = code
            
            // Update status code based on the new error code
            if def, ok := PredefinedErrors[code]; ok {
                octoErr.StatusCode = def.StatusCode
            }
        }
        
        if msg != "" {
            octoErr.Message = msg
        }
        
        // Update caller information
        if pc, file, line, ok := runtime.Caller(1); ok {
            octoErr.file = file
            octoErr.line = line
            if fn := runtime.FuncForPC(pc); fn != nil {
                octoErr.function = fn.Name()
            }
        }
        
        return octoErr
    }
    
    // Create a new OctoError wrapping the original error
    octoErr := New(code, msg)
    octoErr.Original = err
    
    // Update caller information
    if pc, file, line, ok := runtime.Caller(1); ok {
        octoErr.file = file
        octoErr.line = line
        if fn := runtime.FuncForPC(pc); fn != nil {
            octoErr.function = fn.Name()
        }
    }
    
    return octoErr
}

// Wrapf wraps an existing error with a formatted message.
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *OctoError {
    return Wrap(err, code, fmt.Sprintf(format, args...))
}

// Is checks if target is an OctoError with the specified code.
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

// Assert checks a condition and returns an error if the condition is false.
// This implements the "Assert all function preconditions and postconditions" safety rule
// from the coding style guide.
func Assert(condition bool, code ErrorCode, message string) error {
    if !condition {
        return New(code, message)
    }
    return nil
}

// AssertWithError checks a condition and returns the provided error if the condition is false.
func AssertWithError(condition bool, err error) error {
    if !condition {
        return err
    }
    return nil
}

// MustAssert checks a condition and panics if the condition is false.
// Use this only for critical internal assertions that should never fail.
func MustAssert(condition bool, message string) {
    if !condition {
        panic(New(ErrInternal, "Assertion failed: "+message))
    }
}

// LogError logs an error with appropriate context and stack trace.
func LogError(logger *zerolog.Logger, err error) {
    if err == nil || logger == nil {
        return
    }
    
    event := logger.Error().Err(err)
    
    // Add caller info for standard errors
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
        // For OctoErrors, add the specific code and status
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

// LogPanic logs a recovered panic with stack trace.
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