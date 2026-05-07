package render

import (
	"encoding/json"
	"fmt"
	"io"
)

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return &Error{Code: ExitCantOut, Message: fmt.Sprintf("cannot write JSON output: %v", err)}
	}
	return nil
}
