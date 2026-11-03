package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

var nodeKeyOrder = []string{"name", "type", "server", "port"}

// renderClash 输出 Clash 配置：profile 里 clash 段的原文去掉一层缩进后照原样输出，保留注释、空行和字段顺序，
// 生成的 proxies 插在 proxy-groups 之前（没有 proxy-groups 时放在最后）。
func renderClash(src []byte, base map[string]any, nodes []map[string]any) ([]byte, error) {
	for k, v := range base {
		if list, ok := v.([]any); ok && slices.ContainsFunc(list, func(e any) bool { _, ok := e.([]any); return ok }) {
			return nil, fmt.Errorf("clash.%s 里不能嵌套列表", k)
		}
	}

	proxies := make([]any, len(nodes))
	for i, n := range nodes {
		proxies[i] = orderedMap(n, nodeKeyOrder)
	}
	clash := maps.Clone(base)
	if clash == nil {
		clash = make(map[string]any, 1)
	}
	clash["proxies"] = proxies
	if err := checkReferences(clash, nodes); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string]any{"proxies": proxies}); err != nil {
		return nil, err
	}
	proxyText := buf.String()
	if len(base) == 0 {
		return []byte(proxyText), nil
	}

	lines, err := clashLines(src)
	if err != nil {
		return nil, err
	}
	at := len(lines)
	for i, l := range lines {
		if strings.HasPrefix(l, "proxy-groups:") {
			at = i
			// proxy-groups 上方紧挨着的注释跟着它走
			for at > 0 && strings.HasPrefix(lines[at-1], "#") {
				at--
			}
			break
		}
	}
	var out strings.Builder
	for _, l := range lines[:at] {
		out.WriteString(l + "\n")
	}
	if at > 0 && lines[at-1] != "" {
		out.WriteString("\n")
	}
	out.WriteString(proxyText)
	if at < len(lines) {
		out.WriteString("\n")
	}
	for _, l := range lines[at:] {
		out.WriteString(l + "\n")
	}

	// 兜底：输出必须能解析，且内容和校验过的配置一致
	var got map[string]any
	if err := yaml.Unmarshal([]byte(out.String()), &got); err != nil {
		return nil, fmt.Errorf("生成的 Clash 配置无法解析: %w", err)
	}
	if len(got) != len(clash) {
		return nil, errors.New("生成的 Clash 配置字段数不对")
	}
	if ps, _ := got["proxies"].([]any); len(ps) != len(nodes) {
		return nil, errors.New("生成的 Clash 配置节点数不对")
	}
	return []byte(out.String()), nil
}

// clashLines 取出 profile 里 clash 段的原文，去掉一层缩进，末尾的空行和顶格注释（属于下一个字段）去掉。
// 流式写法（clash: {...}）没有可保留的排版，改为重新生成。
func clashLines(src []byte) ([]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	root := doc.Content[0]
	var key, val, next *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "clash" {
			key, val = root.Content[i], root.Content[i+1]
			if i+2 < len(root.Content) {
				next = root.Content[i+2]
			}
			break
		}
	}
	if val.Kind != yaml.MappingNode || val.Style&yaml.FlowStyle != 0 || val.Line == key.Line {
		blockStyle(val)
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(val); err != nil {
			return nil, err
		}
		return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n"), nil
	}

	all := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	end := len(all)
	if next != nil {
		end = next.Line - 1
	}
	lines := all[key.Line:end]
	for len(lines) > 0 {
		last := lines[len(lines)-1]
		if strings.TrimSpace(last) != "" && !strings.HasPrefix(last, "#") {
			break
		}
		lines = lines[:len(lines)-1]
	}
	indent := val.Content[0].Column - 1
	for i, l := range lines {
		if len(l) >= indent && strings.TrimSpace(l[:indent]) == "" {
			lines[i] = l[indent:]
		} else {
			lines[i] = strings.TrimLeft(l, " ")
		}
	}
	return lines, nil
}

// blockStyle 把流式写法改成块写法，方便在中间插入 proxies。
func blockStyle(n *yaml.Node) {
	n.Style &^= yaml.FlowStyle
	for _, c := range n.Content {
		blockStyle(c)
	}
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
