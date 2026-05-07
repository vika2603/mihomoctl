package cli

import (
	"sort"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

type groupOutput struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Selected   string   `json:"selected"`
	Candidates []string `json:"candidates"`
}

func selectableGroups(proxies map[string]mihomo.Proxy) []groupOutput {
	groups := make([]groupOutput, 0)
	for name, p := range proxies {
		if len(p.All) == 0 {
			continue
		}
		if p.Name == "" {
			p.Name = name
		}
		groups = append(groups, groupOutput{
			Name:       p.Name,
			Type:       p.Type,
			Selected:   p.Now,
			Candidates: p.All,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups
}
