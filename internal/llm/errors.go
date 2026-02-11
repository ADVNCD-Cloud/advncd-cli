package llm

import (
	"fmt"
)

type ErrCode string

const (
	ErrConfig  ErrCode = "LLM-CONFIG"
	ErrHTTP    ErrCode = "LLM-HTTP"
	ErrDecode  ErrCode = "LLM-DECODE"
	ErrProvider ErrCode = "LLM-PROVIDER"
)

type Error struct {
	Code    ErrCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func Wrap(code ErrCode, msg string, cause error) error {
	return &Error{Code: code, Message: msg, Cause: cause}
}

func New(code ErrCode, msg string) error {
	return &Error{Code: code, Message: msg}
}
