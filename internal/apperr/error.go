package apperr

import "fmt"

type Error struct {
	Code    string
	Message string
	Cause   error
	Meta    map[string]string
	FixWith []string
}

func New(entry Entry) *Error {
	return &Error{
		Code:    entry.Code,
		Message: entry.Message,
		Meta:    map[string]string{},
	}
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) WithCause(err error) *Error {
	e.Cause = err
	return e
}

func (e *Error) WithMeta(k, v string) *Error {
	if e.Meta == nil {
		e.Meta = map[string]string{}
	}
	e.Meta[k] = v
	return e
}

func (e *Error) WithFix(f string) *Error {
	e.FixWith = append(e.FixWith, f)
	return e
}