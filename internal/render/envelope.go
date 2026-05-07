package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Details  any    `json:"details,omitempty"`
}

func RenderError(err error, jsonMode bool, errOut io.Writer) int {
	var ce *Error
	if errors.As(err, &ce) {
		if ce.Code == ExitOK {
			return ExitOK
		}
		if ce.SuppressRender {
			return ce.Code
		}
		if jsonMode {
			if err := WriteErrorEnvelope(errOut, ce); err != nil {
				fmt.Fprintf(errOut, "cannot write JSON error output: %v\n", err)
				return ExitCantOut
			}
			return ce.Code
		}
		fmt.Fprintln(errOut, ce.Message)
		return ce.Code
	}
	if jsonMode {
		ce = &Error{Code: ExitSoftware, Message: fmt.Sprintf("unexpected error: %v", err), ErrorCode: "internal_error"}
		if err := WriteErrorEnvelope(errOut, ce); err != nil {
			fmt.Fprintf(errOut, "cannot write JSON error output: %v\n", err)
			return ExitCantOut
		}
		return ExitSoftware
	}
	fmt.Fprintf(errOut, "unexpected error: %v\n", err)
	return ExitSoftware
}

func WriteErrorEnvelope(w io.Writer, ce *Error) error {
	body := ToErrorBody(ce)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(ErrorEnvelope{Error: body})
}

func ToErrorBody(err error) ErrorBody {
	var ce *Error
	if !errors.As(err, &ce) {
		return ErrorBody{
			Code:     "internal_error",
			Category: "software",
			Message:  fmt.Sprintf("unexpected error: %v", err),
		}
	}
	code := ce.ErrorCode
	if code == "" {
		code = DefaultErrorCode(ce.Code)
	}
	return ErrorBody{
		Code:     code,
		Category: ErrorCategory(ce.Code),
		Message:  ce.Message,
		Details:  ce.Details,
	}
}

func DefaultErrorCode(exitCode int) string {
	switch exitCode {
	case ExitUsage:
		return "usage_error"
	case ExitNotFound:
		return "not_found"
	case ExitSoftware:
		return "software_error"
	case ExitSystem:
		return "system_error"
	case ExitCantOut:
		return "output_error"
	case ExitTempFail:
		return "controller_unavailable"
	case ExitNoPerm:
		return "auth_failed"
	default:
		return "unknown_error"
	}
}

func ErrorCategory(exitCode int) string {
	switch exitCode {
	case ExitUsage:
		return "usage"
	case ExitNotFound:
		return "not_found"
	case ExitSoftware:
		return "software"
	case ExitSystem:
		return "system"
	case ExitCantOut:
		return "cant_output"
	case ExitTempFail:
		return "tempfail"
	case ExitNoPerm:
		return "noperm"
	default:
		return "software"
	}
}
