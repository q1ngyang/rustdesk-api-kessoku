package starrycontrol

import (
	"errors"
	"fmt"
)

var (
	ErrInstanceNotFound = errors.New("server control instance not found")
	ErrReadOnly         = errors.New("server control is read-only")
	ErrRequestInvalid   = errors.New("server control request is invalid")
	ErrUnavailable      = errors.New("Starry control agent is unavailable")
)

type ProviderError struct {
	Status    int
	Code      string
	Message   string
	RequestID string
	Retryable bool
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("Starry control error %s", e.Code)
}
