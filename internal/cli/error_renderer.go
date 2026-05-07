package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func renderError(err error, jsonMode bool, errOut io.Writer) int {
	var ce *cliError
	if errors.As(err, &ce) {
		if ce.code == exitOK {
			return exitOK
		}
		if jsonMode {
			if err := writeErrorEnvelope(errOut, ce); err != nil {
				fmt.Fprintf(errOut, "cannot write JSON error output: %v\n", err)
				return exitCantOut
			}
			return ce.code
		}
		fmt.Fprintln(errOut, ce.msg)
		return ce.code
	}
	if jsonMode {
		ce = &cliError{code: exitSoftware, msg: fmt.Sprintf("unexpected error: %v", err), errCode: "internal_error"}
		if err := writeErrorEnvelope(errOut, ce); err != nil {
			fmt.Fprintf(errOut, "cannot write JSON error output: %v\n", err)
			return exitCantOut
		}
		return exitSoftware
	}
	fmt.Fprintf(errOut, "unexpected error: %v\n", err)
	return exitSoftware
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Details  any    `json:"details,omitempty"`
}

func writeErrorEnvelope(w io.Writer, ce *cliError) error {
	body := toErrorBody(ce)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(errorEnvelope{Error: body})
}

func toErrorBody(err error) errorBody {
	var ce *cliError
	if !errors.As(err, &ce) {
		return errorBody{
			Code:     "internal_error",
			Category: "software",
			Message:  fmt.Sprintf("unexpected error: %v", err),
		}
	}
	code := ce.errCode
	if code == "" {
		code = defaultErrorCode(ce.code)
	}
	return errorBody{
		Code:     code,
		Category: errorCategory(ce.code),
		Message:  ce.msg,
		Details:  ce.details,
	}
}

func defaultErrorCode(exitCode int) string {
	switch exitCode {
	case exitUsage:
		return "usage_error"
	case exitNotFound:
		return "not_found"
	case exitSoftware:
		return "software_error"
	case exitSystem:
		return "system_error"
	case exitCantOut:
		return "output_error"
	case exitTempFail:
		return "controller_unavailable"
	case exitNoPerm:
		return "auth_failed"
	default:
		return "unknown_error"
	}
}

func errorCategory(exitCode int) string {
	switch exitCode {
	case exitUsage:
		return "usage"
	case exitNotFound:
		return "not_found"
	case exitSoftware:
		return "software"
	case exitSystem:
		return "system"
	case exitCantOut:
		return "cant_output"
	case exitTempFail:
		return "tempfail"
	case exitNoPerm:
		return "noperm"
	default:
		return "software"
	}
}
