package config

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// clashKeyOrder 是输出时顶层字段的顺序，其余字段按字母序排在最后。
var clashKeyOrder = []string{
	"mixed-port", "port", "socks-port", "redir-port", "tproxy-port",
	"allow-lan", "bind-address", "mode", "log-level", "ipv6",
	"external-controller", "secret", "unified-delay", "tcp-concurrent",
	"find-process-mode", "global-client-fingerprint", "geodata-mode",
	"geox-url", "profile", "sniffer", "tun", "hosts", "dns",
	"proxies", "proxy-providers", "proxy-groups", "rule-providers", "rules",
}

var (
	nodeKeyOrder  = []string{"name", "type", "server", "port"}
	groupKeyOrder = []string{"name", "type"}
)

func renderClash(base map[string]any, nodes []map[string]any) ([]byte, error) {
	clash := make(map[string]any, len(base)+1)
	for k, v := range base {
		if s, ok := v.([]any); ok {
			v = flatten(s)
		}
		clash[k] = v
	}

	proxies := make([]any, len(nodes))
	for i, n := range nodes {
		proxies[i] = orderedMap(n, nodeKeyOrder)
	}
	clash["proxies"] = proxies

	if err := checkReferences(clash, nodes); err != nil {
		return nil, err
	}
	if groups, ok := clash["proxy-groups"].([]any); ok {
		ordered := make([]any, len(groups))
		for i, g := range groups {
			ordered[i] = orderedMap(g.(map[string]any), groupKeyOrder)
		}
		clash["proxy-groups"] = ordered
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(orderedMap(clash, clashKeyOrder)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// flatten 展开嵌套列表，方便用 YAML 锚点拼接规则：rules: [*mine, *common]。
func flatten(in []any) []any {
	out := make([]any, 0, len(in))
	for _, v := range in {
		if s, ok := v.([]any); ok {
			out = append(out, flatten(s)...)
			continue
		}
		out = append(out, v)
	}
	return out
}

// builtinPolicies 是 mihomo 内置的策略名。
var builtinPolicies = []string{"DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE"}

// checkReferences 检查代理组成员和规则目标是否都存在，拦住写错名字的情况。
func checkReferences(clash map[string]any, nodes []map[string]any) error {
	known := make(map[string]bool)
	for _, p := range builtinPolicies {
		known[p] = true
	}
	for _, n := range nodes {
		known[n["name"].(string)] = true
	}

	groups, err := listOf(clash, "proxy-groups")
	if err != nil {
		return err
	}
	for i, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			return fmt.Errorf("proxy-groups 第 %d 项不是对象", i+1)
		}
		name, _ := gm["name"].(string)
		if name == "" {
			return fmt.Errorf("proxy-groups 第 %d 项缺少 name", i+1)
		}
		if known[name] {
			return fmt.Errorf("代理组 %s 和节点或其他代理组重名", name)
		}
		known[name] = true
	}
	for _, g := range groups {
		gm := g.(map[string]any)
		members, _ := gm["proxies"].([]any)
		for _, m := range members {
			s, _ := m.(string)
			if !known[s] {
				return fmt.Errorf("代理组 %s 引用了不存在的节点或代理组 %q", gm["name"], m)
			}
		}
	}

	providers, _ := clash["rule-providers"].(map[string]any)
	rules, err := listOf(clash, "rules")
	if err != nil {
		return err
	}
	for _, r := range rules {
		s, ok := r.(string)
		if !ok {
			return fmt.Errorf("规则 %v 不是字符串", r)
		}
		typ, target, payload, ok := parseRule(s)
		if !ok {
			continue
		}
		if target == "" {
			return fmt.Errorf("规则 %q 格式不对", s)
		}
		if !known[target] {
			return fmt.Errorf("规则 %q 的目标 %q 不存在", s, target)
		}
		if typ == "RULE-SET" {
			if _, ok := providers[payload]; !ok {
				return fmt.Errorf("规则 %q 引用的 rule-provider %q 不存在", s, payload)
			}
		}
	}
	return nil
}

// parseRule 取出规则类型、目标和内容；逻辑规则等复杂写法返回 ok=false，不做检查。
func parseRule(s string) (typ, target, payload string, ok bool) {
	fields := strings.Split(s, ",")
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	typ = strings.ToUpper(fields[0])
	switch typ {
	case "AND", "OR", "NOT", "SUB-RULE":
		return typ, "", "", false
	case "MATCH":
		if len(fields) < 2 {
			return typ, "", "", true
		}
		return typ, fields[1], "", true
	}
	if len(fields) < 3 {
		return typ, "", "", true
	}
	return typ, fields[2], fields[1], true
}

func listOf(m map[string]any, key string) ([]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	s, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s 必须是列表", key)
	}
	return s, nil
}

// orderedMap 把 map 转成按指定顺序输出的 yaml.Node，未列出的键按字母序排在后面。
func orderedMap(m map[string]any, first []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		ia, ib := slices.Index(first, a), slices.Index(first, b)
		switch {
		case ia >= 0 && ib >= 0:
			return ia - ib
		case ia >= 0:
			return -1
		case ib >= 0:
			return 1
		}
		return strings.Compare(a, b)
	})
	for _, k := range keys {
		var v yaml.Node
		if n, ok := m[k].(*yaml.Node); ok {
			v = *n
		} else if err := v.Encode(m[k]); err != nil {
			v.Encode(fmt.Sprint(m[k]))
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &v)
	}
	return node
}
