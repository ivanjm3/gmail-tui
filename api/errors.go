package api

// ErrorKind classifies errors as transient (retryable) or permanent (user action needed).
type ErrorKind int

const (
	// ErrTransient indicates a temporary error such as a network timeout or rate limit.
	ErrTransient ErrorKind = iota
	// ErrPermanent indicates a non-recoverable error such as invalid credentials.
	ErrPermanent
)

// AppError is a structured application error with a kind, message, and cause.
type AppError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *AppError) Error() string { return e.Message }

// Unwrap returns the underlying cause for errors.Is/errors.As support.
func (e *AppError) Unwrap() error { return e.Cause }

// NewTransientError creates a transient AppError wrapping cause.
func NewTransientError(msg string, cause error) *AppError {
	return &AppError{Kind: ErrTransient, Message: msg, Cause: cause}
}

// NewPermanentError creates a permanent AppError wrapping cause.
func NewPermanentError(msg string, cause error) *AppError {
	return &AppError{Kind: ErrPermanent, Message: msg, Cause: cause}
}
