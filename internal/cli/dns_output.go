package cli

import (
	"fmt"
	"strings"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

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

func buildDNSOutput(domain, queryType string, resp mihomo.DNSResponse) dnsOutput {
	if len(resp.Questions) > 0 {
		if q := normalizeDomain(resp.Questions[0].Name); q != "" {
			domain = q
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
