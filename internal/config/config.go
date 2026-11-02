// Package config 读取订阅配置目录，并为每个 profile 预先渲染好各客户端的订阅内容。
//
// 目录结构：
//
//	nodes.yaml            全部节点，mihomo proxy 写法的列表
//	profiles/<名字>.yaml   一个订阅一个文件，文件名就是 profile 名
package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

const (
	nodesFile   = "nodes.yaml"
	profilesDir = "profiles"
)

// Config 是加载完成的配置，所有 profile 的订阅内容都已渲染好。
type Config struct {
	Profiles map[string]*Profile
	Unused   []string // 没有被任何 profile 用到的节点
}

// Profile 是一份订阅。
type Profile struct {
	Name    string
	Token   string
	Nodes   []string // 选中的节点名，按 nodes.yaml 中的顺序
	Skipped []string // 无法转换为分享链接而跳过的节点及原因
	Outputs map[Target][]byte
}

type rawProfile struct {
	Token string         `yaml:"token"`
	Nodes []string       `yaml:"nodes"`
	Clash map[string]any `yaml:"clash"`
}

var profileNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Load 读取并校验配置目录。
func Load(dir string) (*Config, error) {
	nodes, err := loadNodes(filepath.Join(dir, nodesFile))
	if err != nil {
		return nil, err
	}
	files, err := profileFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s 下没有 profile", profilesDir)
	}

	cfg := &Config{Profiles: make(map[string]*Profile, len(files))}
	used := make(map[string]bool)
	tokens := make(map[string]string)
	for _, f := range files {
		name := strings.TrimSuffix(f, ".yaml")
		p, err := loadProfile(filepath.Join(dir, profilesDir, f), name, nodes)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", profilesDir, f, err)
		}
		if other, ok := tokens[p.Token]; ok {
			return nil, fmt.Errorf("profile %s 和 %s 的 token 重复", name, other)
		}
		tokens[p.Token] = name
		for _, n := range p.Nodes {
			used[n] = true
		}
		cfg.Profiles[name] = p
	}
	for _, n := range nodes {
		if name := n["name"].(string); !used[name] {
			cfg.Unused = append(cfg.Unused, name)
		}
	}
	return cfg, nil
}

// Fingerprint 返回配置目录内所有配置文件的修改时间和大小，用来判断是否需要重载。
func Fingerprint(dir string) (string, error) {
	paths := []string{filepath.Join(dir, nodesFile)}
	files, err := os.ReadDir(filepath.Join(dir, profilesDir))
	if err != nil {
		return "", err
	}
	for _, f := range files {
		paths = append(paths, filepath.Join(dir, profilesDir, f.Name()))
	}
	var b strings.Builder
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%s %d %d\n", p, fi.ModTime().UnixNano(), fi.Size())
	}
	return b.String(), nil
}

// profileFiles 列出 profiles 目录下的 profile 文件；以 . 开头的文件忽略，其余不合规的文件报错。
func profileFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, profilesDir))
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			return nil, fmt.Errorf("%s 下不能有子目录: %s", profilesDir, name)
		}
		base, ok := strings.CutSuffix(name, ".yaml")
		if !ok || !profileNameRe.MatchString(base) {
			return nil, fmt.Errorf("%s/%s: 文件名只能用小写字母、数字和短横线，扩展名为 .yaml", profilesDir, name)
		}
		files = append(files, name)
	}
	return files, nil
}

func decodeStrict(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("解析 YAML: %w", err)
	}
	return nil
}

func loadNodes(path string) ([]map[string]any, error) {
	var nodes []map[string]any
	if err := decodeStrict(path, &nodes); err != nil {
		return nil, fmt.Errorf("%s: %w", nodesFile, err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("%s: 没有节点", nodesFile)
	}
	seen := make(map[string]bool, len(nodes))
	for i, n := range nodes {
		name, _ := n["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("%s: 第 %d 个节点缺少 name", nodesFile, i+1)
		}
		if seen[name] {
			return nil, fmt.Errorf("%s: 节点名重复: %s", nodesFile, name)
		}
		seen[name] = true
		for _, k := range []string{"type", "server", "port"} {
			if _, ok := n[k]; !ok {
				return nil, fmt.Errorf("%s: 节点 %s 缺少 %s", nodesFile, name, k)
			}
		}
	}
	return nodes, nil
}

func loadProfile(path, name string, nodes []map[string]any) (*Profile, error) {
	var rp rawProfile
	if err := decodeStrict(path, &rp); err != nil {
		return nil, err
	}
	if len(rp.Token) < 16 {
		return nil, errors.New("token 至少 16 个字符")
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

// selectNodes 按节点名挑选节点，结果按 nodes.yaml 中的顺序；不写表示全部节点。
func selectNodes(names []string, nodes []map[string]any) ([]map[string]any, error) {
	if len(names) == 0 {
		return nodes, nil
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		if want[n] {
			return nil, fmt.Errorf("nodes 里 %q 重复", n)
		}
		want[n] = true
	}
	var out []map[string]any
	for _, n := range nodes {
		name := n["name"].(string)
		if want[name] {
			out = append(out, n)
			delete(want, name)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for n := range want {
			missing = append(missing, n)
		}
		slices.Sort(missing)
		return nil, fmt.Errorf("nodes 里的节点不存在: %s", strings.Join(missing, ", "))
	}
	return out, nil
}
