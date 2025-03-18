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
    logErrorInternal(logger, err, "", "")
}

// LogErrorWithPath logs errors with request path context
func LogErrorWithPath(logger *zerolog.Logger, err error, path string) {
    logErrorInternal(logger, err, path, "")
}

// LogErrorWithPathIP logs errors with request path and IP address context
func LogErrorWithPathIP(logger *zerolog.Logger, err error, path string, ip string) {
    logErrorInternal(logger, err, path, ip)
}

// Internal function for error logging, supporting optional path and IP context
func logErrorInternal(logger *zerolog.Logger, err error, path string, ip string) {
    if err == nil || logger == nil {
        return
    }
    
    event := logger.Error().Err(err)
    
    // Add request path if available
    if path != "" {
        event = event.Str("path", path)
    }
    
    // Add client IP if available
    if ip != "" {
        event = event.Str("ip", ip)
    }
    
    // Add caller information based on error type
    if _, ok := err.(*OctoError); !ok {
        if pc, file, line, ok := runtime.Caller(2); ok {
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
    LogPanicWithRequestInfo(logger, recovered, stack, "", "", "")
}

// Handles and logs recovered panics with full context including request information
func LogPanicWithRequestInfo(logger *zerolog.Logger, recovered interface{}, stack []byte, path string, method string, ip string) {
    if logger == nil {
        return
    }
    
    var err error
    var errMsg string
    
    // Extract a meaningful panic message
    switch v := recovered.(type) {
    case error:
        err = v
        errMsg = v.Error()
    case string:
        err = errors.New(v)
        errMsg = v
    default:
        errMsg = fmt.Sprintf("%v", recovered)
        err = errors.New(errMsg)
    }
    
    wrappedErr := Wrap(err, ErrInternal, "Panic recovered")
    
    // Convert stack trace format to match the router's format
    stackStr := string(stack)
    stackLines := strings.Split(stackStr, "\n")
    
    // Extract the panic location (function, file, line) from the stack trace
    var panicLocation string
    if len(stackLines) >= 2 {
        panicFunc := strings.TrimSpace(stackLines[1])
        panicFile := strings.TrimSpace(stackLines[2])
        panicLocation = fmt.Sprintf("%s at %s", panicFunc, panicFile)
    }
    
    // Format the stack trace for better readability
    zStack := make([]string, 0, len(stackLines)/2)
    for i := 1; i < len(stackLines); i += 2 {
        if i+1 < len(stackLines) {
            funcLine := strings.TrimSpace(stackLines[i])
            fileLine := strings.TrimSpace(stackLines[i+1])
            if funcLine != "" && fileLine != "" {
                // Clean up the function and file information for better readability
                funcName := funcLine
                // Extract just the function name without the package path if possible
                if lastDot := strings.LastIndex(funcName, "."); lastDot > 0 {
                    funcBaseName := funcName[lastDot+1:]
                    if len(funcBaseName) > 0 {
                        // Keep package name for context but highlight the function name
                        funcName = funcName[:lastDot+1] + funcBaseName
                    }
                }
                zStack = append(zStack, funcName+"\n\t"+fileLine)
            }
        }
    }
    
    // Create the log event
    event := logger.Error().
        Err(wrappedErr).
        Str("error_code", string(wrappedErr.Code)).
        Int("status_code", wrappedErr.StatusCode)
    
    // Add a summary section for quick understanding
    requestInfo := ""
    if method != "" && path != "" {
        requestInfo = fmt.Sprintf(" during %s %s", method, path)
    }
    
    clientInfo := ""
    if ip != "" {
        clientInfo = fmt.Sprintf(" from %s", ip)
    }
    
    event = event.Str("panic_summary", fmt.Sprintf("%s%s%s", errMsg, requestInfo, clientInfo))
    
    if panicLocation != "" {
        event = event.Str("panic_location", panicLocation)
    }
    
    // Add stack array in the format expected by the router
    stackArr := zerolog.Arr()
    for _, s := range zStack {
        stackArr = stackArr.Str(s)
    }
    event = event.Array("stack_array", stackArr)
    
    // Add request information if available
    if path != "" {
        event = event.Str("path", path)
    }
    if method != "" {
        event = event.Str("method", method)
    }
    if ip != "" {
        event = event.Str("ip", ip)
    }
    
    // Create a more informative message
    logMsg := "[octo-panic] Panic recovered"
    if panicLocation != "" {
        logMsg = fmt.Sprintf("[octo-panic] Panic recovered: %s at %s", errMsg, panicLocation)
    } else {
        logMsg = fmt.Sprintf("[octo-panic] Panic recovered: %s", errMsg)
    }
    
    event.Msg(logMsg)
}