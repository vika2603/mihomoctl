package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

type dnsQueryOptions struct {
	queryType string
}

type dnsOutput struct {
	Domain    string      `json:"domain"`
	QueryType string      `json:"query_type"`
	Status    string      `json:"status"`
	Answers   []dnsAnswer `json:"answers"`
}

type dnsAnswer struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  uint32 `json:"ttl"`
	Data string `json:"data"`
}

func newDNSCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Debug mihomo DNS resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("dns requires query")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown dns subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newDNSQueryCommand(out, cfg))
	return cmd
}

func newDNSQueryCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := dnsQueryOptions{queryType: "A"}
	cmd := &cobra.Command{
		Use:   "query <domain>",
		Short: "Resolve a domain through mihomo DNS",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("dns query requires <domain>")
			}
			if strings.TrimSpace(args[0]) == "" {
				return usage("domain cannot be empty")
			}
			if strings.TrimSpace(opts.queryType) == "" {
				return usage("--type cannot be empty")
			}
			if !validDNSQueryType(opts.queryType) {
				return usage("unsupported DNS query type %q", opts.queryType)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runDNSQuery(ctx, out, *cfg, client, args[0], opts)
			})
		},
	}
	cmd.Flags().StringVar(&opts.queryType, "type", opts.queryType, "DNS query type, for example A, AAAA, TXT, or CNAME")
	return cmd
}

func runDNSQuery(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, domain string, opts dnsQueryOptions) error {
	resp, err := client.QueryDNS(ctx, domain, strings.ToUpper(opts.queryType))
	if err != nil {
		return mapErr(err)
	}
	result := buildDNSOutput(domain, strings.ToUpper(opts.queryType), resp)
	if cfg.jsonOut {
		return writeJSON(out, result)
	}
	fmt.Fprintln(out, "domain\tquery_type\tstatus")
	fmt.Fprintf(out, "%s\t%s\t%s\n", result.Domain, result.QueryType, result.Status)
	if len(result.Answers) == 0 {
		fmt.Fprintln(out, "no answers")
		return nil
	}
	fmt.Fprintln(out, "name\ttype\tttl\tdata")
	for _, answer := range result.Answers {
		fmt.Fprintf(out, "%s\t%s\t%d\t%s\n", answer.Name, answer.Type, answer.TTL, answer.Data)
	}
	return nil
}

func buildDNSOutput(domain, queryType string, resp mihomo.DNSResponse) dnsOutput {
	if len(resp.Questions) > 0 {
		if q := normalizeDomain(resp.Questions[0].Name); q != "" {
			domain = q
		}
		if typ := dnsTypeName(resp.Questions[0].Type); typ != "" {
			queryType = typ
		}
	}
	answers := make([]dnsAnswer, 0, len(resp.Answers))
	for _, answer := range resp.Answers {
		answers = append(answers, dnsAnswer{
			Name: normalizeDomain(answer.Name),
			Type: dnsTypeName(answer.Type),
			TTL:  answer.TTL,
			Data: answer.Data,
		})
	}
	return dnsOutput{
		Domain:    normalizeDomain(domain),
		QueryType: queryType,
		Status:    dnsStatusName(resp.Status),
		Answers:   answers,
	}
}

func normalizeDomain(domain string) string {
	return strings.TrimSuffix(domain, ".")
}

func dnsStatusName(code int) string {
	names := map[int]string{
		0: "NOERROR",
		1: "FORMERR",
		2: "SERVFAIL",
		3: "NXDOMAIN",
		4: "NOTIMP",
		5: "REFUSED",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return fmt.Sprintf("RCODE%d", code)
}

func dnsTypeName(code uint16) string {
	names := map[uint16]string{
		1:  "A",
		2:  "NS",
		5:  "CNAME",
		6:  "SOA",
		12: "PTR",
		15: "MX",
		16: "TXT",
		28: "AAAA",
		33: "SRV",
		65: "HTTPS",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return fmt.Sprintf("TYPE%d", code)
}

func validDNSQueryType(queryType string) bool {
	switch strings.ToUpper(strings.TrimSpace(queryType)) {
	case "A", "AAAA", "CNAME", "HTTPS", "MX", "NS", "PTR", "SOA", "SRV", "TXT":
		return true
	default:
		return false
	}
}
