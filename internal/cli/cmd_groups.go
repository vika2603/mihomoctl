package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type groupsOutput struct {
	Total  int           `json:"total"`
	Groups []groupOutput `json:"groups"`
}

func newGroupsCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Inspect proxy groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("groups requires list or get")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown groups subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newGroupsListCommand(out, cfg), newGroupsGetCommand(out, cfg))
	return cmd
}

func newGroupsListCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proxy groups",
		Long:  "List proxy groups.\n\nInventory: GET /group.\nThis is a read-only command.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("groups list takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runGroupsList(ctx, out, *cfg, client)
			})
		},
	}
	return cmd
}

func newGroupsGetCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show one proxy group",
		Long:  "Show one proxy group.\n\nInventory: GET /group/{name}.\nThis is a read-only command.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("groups get requires <name>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runGroupsGet(ctx, out, *cfg, client, args[0])
			})
		},
	}
	return cmd
}

func runGroupsList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client) error {
	groups, err := client.ListGroups(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildGroupsOutput(groups)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	if len(result.Groups) == 0 {
		fmt.Fprintln(out, "no proxy groups")
		return nil
	}
	fmt.Fprintln(out, "name\ttype\tselected\tcandidates")
	for _, g := range result.Groups {
		fmt.Fprintf(out, "%s\t%s\t%s\t%d\n", g.Name, g.Type, emptyDash(g.Selected), len(g.Candidates))
	}
	return nil
}

func runGroupsGet(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, name string) error {
	group, err := client.GetGroup(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	result := buildGroupOutput(group.Name, group)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintf(out, "%s (%s)\n", result.Name, result.Type)
	fmt.Fprintf(out, "selected: %s\n", emptyDash(result.Selected))
	if len(result.Candidates) == 0 {
		fmt.Fprintln(out, "candidates: -")
		return nil
	}
	fmt.Fprintln(out, "candidates:")
	for _, node := range result.Candidates {
		marker := " "
		if node == result.Selected {
			marker = "*"
		}
		fmt.Fprintf(out, "  %s %s\n", marker, node)
	}
	return nil
}

func buildGroupsOutput(groups []mihomo.Proxy) groupsOutput {
	rows := make([]groupOutput, 0, len(groups))
	for _, group := range groups {
		if len(group.All) == 0 {
			continue
		}
		rows = append(rows, buildGroupOutput(group.Name, group))
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return groupsOutput{Total: len(rows), Groups: rows}
}

func buildGroupOutput(name string, proxy mihomo.Proxy) groupOutput {
	if proxy.Name != "" {
		name = proxy.Name
	}
	return groupOutput{
		Name:       name,
		Type:       proxy.Type,
		Selected:   proxy.Now,
		Candidates: append([]string(nil), proxy.All...),
	}
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
