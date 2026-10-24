package errs

import "fmt"

// Error is a typed failure the CLI can turn into the agent schema.
type Error struct {
	Code   string
	Status int
	Msg    string
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("Twitter API error (HTTP %d): %s", e.Status, e.Msg)
	}
	return e.Msg
}

func New(code, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

func Auth(msg string) *Error { return New("not_authenticated", msg) }

func NotFound(msg string) *Error { return New("not_found", msg) }

func Network(msg string) *Error { return New("network_error", msg) }

func QueryID(msg string) *Error { return New("query_id_error", msg) }

func Media(msg string) *Error { return New("media_upload_error", msg) }

func Input(msg string) *Error { return New("invalid_input", msg) }

func API(status int, msg string) *Error {
	code := "api_error"
	switch status {
	case 401, 403:
		code = "not_authenticated"
	case 429:
		code = "rate_limited"
	case 404:
		code = "not_found"
	}
	return &Error{Code: code, Status: status, Msg: msg}
}

// Code returns the schema error code for any error.
func Code(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok && e.Code != "" {
		return e.Code
	}
	return "api_error"
}
