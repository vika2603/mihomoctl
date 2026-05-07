package render

import (
	"errors"
	"fmt"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

const (
	ExitOK       = 0
	ExitUsage    = 64
	ExitNotFound = 66
	ExitSoftware = 70
	ExitSystem   = 71
	ExitCantOut  = 73
	ExitTempFail = 75
	ExitNoPerm   = 77
)

type Error struct {
	Code           int
	Message        string
	ErrorCode      string
	Details        any
	SuppressRender bool
}

func (e *Error) Error() string { return e.Message }

func Usage(format string, a ...any) error {
	return &Error{Code: ExitUsage, Message: fmt.Sprintf(format, a...)}
}

func NewError(code int, message, errorCode string, details any) *Error {
	return &Error{Code: code, Message: message, ErrorCode: errorCode, Details: details}
}

func MapMihomoError(err error) error {
	var me *mihomo.Error
	if !errors.As(err, &me) {
		return &Error{Code: ExitSoftware, Message: err.Error()}
	}
	switch me.Kind {
	case mihomo.ErrAuth:
		return &Error{Code: ExitNoPerm, Message: me.Msg}
	case mihomo.ErrBadRequest:
		return &Error{Code: ExitUsage, Message: me.Msg}
	case mihomo.ErrNotFound:
		return &Error{Code: ExitNotFound, Message: me.Msg}
	case mihomo.ErrUnavailable:
		return &Error{Code: ExitTempFail, Message: me.Msg}
	default:
		return &Error{Code: ExitSoftware, Message: me.Msg}
	}
}
