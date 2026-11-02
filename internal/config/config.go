// Package config 读取订阅配置，并为每个 profile 预先渲染好各客户端的订阅内容。
package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"freedom/internal/link"
)

// Target 是订阅输出格式。
type Target string

const (
	// TargetClash 是 mihomo（Clash Meta）完整配置。
	TargetClash Target = "clash"
	// TargetV2ray 是 base64 编码的分享链接列表，供 v2rayNG、Shadowrocket 等使用。
	TargetV2ray Target = "v2ray"
)

// ParseTarget 把字符串解析为 Target。
func ParseTarget(s string) (Target, bool) {
	switch Target(s) {
	case TargetClash, TargetV2ray:
		return Target(s), true
	}
	return "", false
}

// Config 是加载完成的配置，所有 profile 的订阅内容都已渲染好。
type Config struct {
	Profiles map[string]*Profile
}

// Profile 是一份订阅（通常对应一台设备）。
type Profile struct {
	Name    string
	Token   string
	Nodes   []string // 选中的节点名，按配置顺序
	Skipped []string // 无法转换为分享链接而跳过的节点及原因
	Outputs map[Target][]byte
}

type rawConfig struct {
	Nodes    []map[string]any       `yaml:"nodes"`
	Profiles map[string]*rawProfile `yaml:"profiles"`
	Extra    map[string]any         `yaml:",inline"`
}

type rawProfile struct {
	Token string         `yaml:"token"`
	Nodes []string       `yaml:"nodes"`
	Clash map[string]any `yaml:"clash"`
}

var profileNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Load 读取并校验配置文件。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse 解析并校验配置内容。
func Parse(data []byte) (*Config, error) {
	var raw rawConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析 YAML: %w", err)
	}
	for k := range raw.Extra {
		if !strings.HasPrefix(k, "x-") {
			return nil, fmt.Errorf("未知的顶层字段 %q（自定义锚点请用 x- 开头）", k)
		}
	}

	nodes, err := checkNodes(raw.Nodes)
	if err != nil {
		return nil, err
	}
	if len(raw.Profiles) == 0 {
		return nil, errors.New("profiles 不能为空")
	}

	cfg := &Config{Profiles: make(map[string]*Profile, len(raw.Profiles))}
	tokens := make(map[string]string)
	for name, rp := range raw.Profiles {
		p, err := buildProfile(name, rp, nodes)
		if err != nil {
			return nil, fmt.Errorf("profile %s: %w", name, err)
		}
		if other, ok := tokens[p.Token]; ok {
			return nil, fmt.Errorf("profile %s 和 %s 的 token 重复", name, other)
		}
		tokens[p.Token] = name
		cfg.Profiles[name] = p
	}
	return cfg, nil
}

func checkNodes(nodes []map[string]any) ([]map[string]any, error) {
	if len(nodes) == 0 {
		return nil, errors.New("nodes 不能为空")
	}
	seen := make(map[string]bool, len(nodes))
	for i, n := range nodes {
		name, _ := n["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("第 %d 个节点缺少 name", i+1)
		}
		if seen[name] {
			return nil, fmt.Errorf("节点名重复: %s", name)
		}
		seen[name] = true
		for _, k := range []string{"type", "server", "port"} {
			if _, ok := n[k]; !ok {
				return nil, fmt.Errorf("节点 %s 缺少 %s", name, k)
			}
		}
	}
	return nodes, nil
}

func buildProfile(name string, rp *rawProfile, nodes []map[string]any) (*Profile, error) {
	if !profileNameRe.MatchString(name) {
		return nil, errors.New("名字只能用小写字母、数字和短横线")
	}
	if rp == nil {
		return nil, errors.New("内容为空")
	}
	if len(rp.Token) < 16 {
		return nil, errors.New("token 至少 16 个字符")
	}
	if rp.Clash == nil {
		return nil, errors.New("缺少 clash 配置")
	}
	if _, ok := rp.Clash["proxies"]; ok {
		return nil, errors.New("clash 里不要写 proxies，节点由 nodes 选择")
	}

	selected, err := selectNodes(rp.Nodes, nodes)
	if err != nil {
		return nil, err
	}

	p := &Profile{Name: name, Token: rp.Token, Outputs: make(map[Target][]byte, 2)}
	for _, n := range selected {
		p.Nodes = append(p.Nodes, n["name"].(string))
	}

	clash, err := renderClash(rp.Clash, selected)
	if err != nil {
		return nil, err
	}
	p.Outputs[TargetClash] = clash

	var links []string
	for _, n := range selected {
		l, err := link.Encode(n)
		if err != nil {
			p.Skipped = append(p.Skipped, fmt.Sprintf("%s: %v", n["name"], err))
			continue
		}
		links = append(links, l)
	}
	p.Outputs[TargetV2ray] = []byte(base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n"))))
	return p, nil
}

// selectNodes 按正则（全名匹配）挑选节点；不写表示全部节点。每个正则都必须至少匹配一个节点。
func selectNodes(patterns []string, nodes []map[string]any) ([]map[string]any, error) {
	if len(patterns) == 0 {
		return nodes, nil
	}
	res := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		re, err := regexp.Compile(`^(?:` + p + `)$`)
		if err != nil {
			return nil, fmt.Errorf("nodes 正则 %q: %w", p, err)
		}
		res[i] = re
	}
	hit := make([]bool, len(res))
	var out []map[string]any
	for _, n := range nodes {
		name := n["name"].(string)
		matched := false
		for i, re := range res {
			if re.MatchString(name) {
				hit[i] = true
				matched = true
			}
		}
		if matched {
			out = append(out, n)
		}
	}
	for i, ok := range hit {
		if !ok {
			return nil, fmt.Errorf("nodes 正则 %q 没有匹配任何节点", patterns[i])
		}
	}
	return out, nil
}
