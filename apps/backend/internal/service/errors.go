package service

import "errors"

// Error is a safe message that can be returned to the administrator.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// E builds an API error.
func E(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// As reports whether err is an API error.
func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
