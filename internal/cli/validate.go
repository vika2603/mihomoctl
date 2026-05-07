package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

func validateSelection(proxies map[string]mihomo.Proxy, group, node string) (string, error) {
	p, ok := proxies[group]
	if !ok || len(p.All) == 0 {
		available := make([]string, 0, len(proxies))
		for name, p := range proxies {
			if len(p.All) > 0 {
				available = append(available, name)
			}
		}
		sort.Strings(available)
		return "", &cliError{code: exitNotFound, msg: fmt.Sprintf("group %q not found, available: %s", group, strings.Join(available, ", "))}
	}
	for _, candidate := range p.All {
		if candidate == node {
			return p.Now, nil
		}
	}
	return "", &cliError{code: exitNotFound, msg: fmt.Sprintf("node %q not found in group %q, available: %s", node, group, strings.Join(p.All, ", "))}
}

func validateDelayGroup(proxies map[string]mihomo.Proxy, group string) (mihomo.Proxy, error) {
	p, ok := proxies[group]
	if !ok {
		return mihomo.Proxy{}, &cliError{code: exitNotFound, msg: fmt.Sprintf("group %q not found, available: %s", group, strings.Join(delayGroupNames(proxies), ", "))}
	}
	if !delaySupportedType(p.Type) {
		return mihomo.Proxy{}, usage("group %q type %q does not support delay test, applicable types: URLTest, Selector, Fallback, LoadBalance", group, p.Type)
	}
	if len(p.All) == 0 {
		return mihomo.Proxy{}, &cliError{code: exitNotFound, msg: fmt.Sprintf("group %q not found, available: %s", group, strings.Join(delayGroupNames(proxies), ", "))}
	}
	return p, nil
}

func delayGroupNames(proxies map[string]mihomo.Proxy) []string {
	available := make([]string, 0, len(proxies))
	for name, p := range proxies {
		if delaySupportedType(p.Type) && len(p.All) > 0 {
			if p.Name != "" {
				name = p.Name
			}
			available = append(available, name)
		}
	}
	sort.Strings(available)
	return available
}

func delaySupportedType(typ string) bool {
	switch typ {
	case "URLTest", "Selector", "Fallback", "LoadBalance":
		return true
	default:
		return false
	}
}
