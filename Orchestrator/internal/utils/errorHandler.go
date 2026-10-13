package utils

import (
	"fmt"
)

// errorHandler is wrapper to print the error for fxn
type ErrorHandler struct {
	Pkg string
	Fxn string // hold the function name for the all error
}

// Perror return the error with function name added
func (h *ErrorHandler) Perror(err error) error {
	return fmt.Errorf("%v.%v: %w", h.Pkg, h.Fxn, err)
}

// Serror take a string msg and return error
func (h *ErrorHandler) Serror(msg string) error {
	return fmt.Errorf("%s.%s: %s", h.Pkg, h.Fxn, msg)
}

// Errorf return the error in fmt.Errorf form
func (h *ErrorHandler) Errorf(format string, a ...any) error {
	return fmt.Errorf("%v.%v: %w", h.Pkg, h.Fxn, fmt.Errorf(format, a...))
}
