package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newManCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "man <directory>",
		Short:  "Generate mihomoctl man pages",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("man requires <directory>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateManPages(args[0])
		},
	}
	return cmd
}

func generateManPages(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &cliError{code: exitCantOut, msg: fmt.Sprintf("cannot create man page directory: %v", err)}
	}
	root := newRootCommand(os.Stdout)
	root.DisableAutoGenTag = true
	if err := doc.GenManTree(root, &doc.GenManHeader{Title: "MIHOMOCTL", Section: "1"}, dir); err != nil {
		return &cliError{code: exitCantOut, msg: fmt.Sprintf("cannot generate man pages: %v", err)}
	}
	return nil
}
