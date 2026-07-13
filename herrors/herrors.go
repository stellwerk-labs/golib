package herrors

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// PlatformOrchestratorError represents a standard platform-orchestrator Error
type PlatformOrchestratorError struct {
	StatusCode int   `json:"-"`
	Err        error `json:"-"`
	// Code is a short machine-readable indication of the error. eg (HTTP-123)
	Code string `json:"error"`
	// Message is the human-readable description of the reason for the error.
	Message string `json:"message"`
	// Details is optional and carries any additional detail for the error.
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error allows the PlatformOrchestratorError struct to be used as an error object.
func (e *PlatformOrchestratorError) Error() string {
	errorText := fmt.Sprintf("%s: %s", e.HTTPErrorCode(), e.APIErrorMessage())
	if nil != e.Err {
		errorText += ": " + e.Err.Error()
	}
	return errorText
}

// Unwrap allows the PlatformOrchestratorError struct to be used with go built in errors.Is() and errors.As()
func (e *PlatformOrchestratorError) Unwrap() error {
	return e.Err
}

// APIErrorMessage returns the API error message if set or generates one following the API guidelines
func (e *PlatformOrchestratorError) APIErrorMessage() string {
	message := e.Message
	if message == "" {
		// for legacy reasons we use this message for internal errors
		if e.StatusCode == 500 || e.StatusCode == 0 {
			message = "Unexpected error"
		} else {
			message = http.StatusText(e.StatusCode)
		}
	}
	return message
}

// HTTPErrorCode returns the HTTP error code if set or generates one following the API guidelines
func (e *PlatformOrchestratorError) HTTPErrorCode() string {
	code := e.Code
	if code == "" {
		code = fmt.Sprintf("HTTP-%d", e.StatusCode)
	}
	return code
}

// WithConventions returns a copy of the error with the code and message formatted according to the API conventions.
func (e *PlatformOrchestratorError) WithConventions() *PlatformOrchestratorError {
	herr := *e
	herr.Code = e.HTTPErrorCode()
	herr.Message = e.APIErrorMessage()
	return &herr
}

// WriteToHttp writes the error to an http response following the API conventions.
//
// Error information will be printed to the standard go logger. Developers should use WriteToHttpWithErrLogger for
// more control.
func (e *PlatformOrchestratorError) WriteToHttp(w http.ResponseWriter) {
	e.WriteToHttpWithErrLogger(w, func(s string, i ...interface{}) {
		log.Printf("%s %v", s, i)
	})
}

// WriteToHttpWithErrLogger writes the error to an http response following the API conventions. This is only needed for
// servers not using the hecho package.
//
// A logger function such as zap.SugaredLogger.Errorw should be provided for best results. Ideally
// this logger also holds tracing information that correlates these logs to the request.
// If no logger function is provided, no logs will be sent.
func (e *PlatformOrchestratorError) WriteToHttpWithErrLogger(w http.ResponseWriter, logger func(string, ...interface{})) {
	// it's easy to lose this error and never print it anywhere useful, lets make sure we emit it here into the logs
	// using the provided log function
	if e.Err != nil && logger != nil {
		logger("writing error response with go error", "err", e.Err)
	}
	// emit the header
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(e.StatusCode)
	// marshal the body, this may fail if the connection closes, or the details are not marshal-able
	if err := json.NewEncoder(w).Encode(e.WithConventions()); err != nil && logger != nil {
		logger("failed to marshal response", "err", err)
	}
}

// New wraps an error and creates a new platform-orchestrator Error.
func New(code, message string, details map[string]interface{}, err error) *PlatformOrchestratorError {
	return &PlatformOrchestratorError{
		Err:     err,
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewWithStatusAndDetails constructs an appropriate PlatformOrchestratorError from a status, optional message, details, and error.
// If the 'code' needs to be overridden this should be set after construction.
func NewWithStatusAndDetails(status int, message string, optionalDetails map[string]interface{}, optionalErr error) *PlatformOrchestratorError {
	herr := &PlatformOrchestratorError{
		StatusCode: status,
		Message:    message,
		Details:    optionalDetails,
		Err:        optionalErr,
	}
	return herr.WithConventions()
}

// NewWithStatus constructs an appropriate PlatformOrchestratorError from a status, optional message, and optional error.
// If the 'code' needs to be overridden this should be set after construction.
func NewWithStatus(status int, message string, optionalErr error) *PlatformOrchestratorError {
	return NewWithStatusAndDetails(status, message, nil, optionalErr)
}

// NewInternalError constructs an appropriate PlatformOrchestratorError from a generic golang error.
// If the 'code' needs to be overridden this should be set after construction.
func NewInternalError(err error) *PlatformOrchestratorError {
	return NewWithStatus(http.StatusInternalServerError, "", err)
}
