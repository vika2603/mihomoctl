package cli

import (
	"errors"
	"fmt"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

type cliError struct {
	code    int
	msg     string
	errCode string
	details any
}

func (e *cliError) Error() string { return e.msg }

func usage(format string, a ...any) error {
	return &cliError{code: exitUsage, msg: fmt.Sprintf(format, a...)}
}

func mapErr(err error) error {
	var me *mihomo.Error
	if !errors.As(err, &me) {
		return &cliError{code: exitSoftware, msg: err.Error()}
	}
	switch me.Kind {
	case mihomo.ErrAuth:
		return &cliError{code: exitNoPerm, msg: me.Msg}
	case mihomo.ErrBadRequest:
		return &cliError{code: exitUsage, msg: me.Msg}
	case mihomo.ErrNotFound:
		return &cliError{code: exitNotFound, msg: me.Msg}
	case mihomo.ErrUnavailable:
		return &cliError{code: exitTempFail, msg: me.Msg}
	default:
		return &cliError{code: exitSoftware, msg: me.Msg}
	}
}
