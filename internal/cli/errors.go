package cli

import "github.com/the-super-company/mihomoctl/internal/render"

type cliError = render.Error

func usage(format string, a ...any) error {
	return render.Usage(format, a...)
}

func mapErr(err error) error {
	return render.MapMihomoError(err)
}
