package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestExample(t *testing.T) {
	cfg, err := Load("../../example")
	if err != nil {
		t.Fatalf("示例配置无法加载: %v", err)
	}

	mac := cfg.Profiles["alice-mac"]
	if got := strings.Join(mac.Nodes, "|"); got != "hk-01|jp-01|hk-01 => jp-01" {
		t.Errorf("alice-mac 节点 = %s", got)
	}
	android := cfg.Profiles["alice-android"]
	if got := strings.Join(android.Nodes, "|"); got != "hk-01|hk-01 => jp-01" {
		t.Errorf("alice-android 节点 = %s", got)
	}
	if len(cfg.Unused) != 0 {
		t.Errorf("示例里不应有未使用的节点: %v", cfg.Unused)
	}

	var clash map[string]any
	if err := yaml.Unmarshal(mac.Outputs[TargetClash], &clash); err != nil {
		t.Fatal(err)
	}
	if rules := clash["rules"].([]any); len(rules) != 5 || rules[4] != "MATCH,proxy" {
		t.Errorf("规则 = %v", rules)
	}
	jp := clash["proxies"].([]any)[1].(map[string]any)
	if jp["reality-opts"].(map[string]any)["short-id"] != "0123" {
		t.Errorf("锚点没有展开: %v", jp)
	}
	if !strings.HasPrefix(string(mac.Outputs[TargetClash]), "mixed-port:") {
		t.Errorf("顶层字段顺序不对:\n%s", mac.Outputs[TargetClash])
	}

	// 不写 clash 时只输出节点
	var bare map[string]any
	if err := yaml.Unmarshal(android.Outputs[TargetClash], &bare); err != nil {
		t.Fatal(err)
	}
	if len(bare) != 1 || len(bare["proxies"].([]any)) != 2 {
		t.Errorf("alice-android 的 Clash 输出应只有 2 个节点: %v", bare)
	}

	links, err := base64.StdEncoding.DecodeString(string(android.Outputs[TargetV2ray]))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(string(links), "\n")); n != 2 {
		t.Errorf("alice-android 分享链接数 = %d", n)
	}
}

const testNodes = `
- {name: a, type: vless, server: 1.1.1.1, port: 443, uuid: x}
- {name: b(香港), type: vless, server: 1.1.1.1, port: 443, uuid: y}
`

// writeDir 写出一个配置目录，files 的键是相对路径。
func writeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := files["nodes.yaml"]; !ok {
		files["nodes.yaml"] = testNodes
	}
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestNodesAndFiles(t *testing.T) {
	dir := writeDir(t, map[string]string{
		"profiles/p.yaml":         "token: 0123456789abcdef\nnodes: [b(香港)]\n",
		"profiles/._p.yaml":       "garbage",
		"profiles/.hidden/x.yaml": "garbage",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Profiles["p"].Nodes; len(got) != 1 || got[0] != "b(香港)" {
		t.Errorf("节点名应按原样精确匹配: %v", got)
	}
	if len(cfg.Unused) != 1 || cfg.Unused[0] != "a" {
		t.Errorf("未使用节点 = %v", cfg.Unused)
	}
}

func TestErrors(t *testing.T) {
	const tok = "token: 0123456789abcdef\n"
	cases := map[string]map[string]string{
		"没有 profile":        {},
		"节点缺字段":             {"nodes.yaml": "- {name: a, type: vless}", "profiles/p.yaml": tok},
		"节点重名":              {"nodes.yaml": testNodes + "- {name: a, type: vless, server: h, port: 1}", "profiles/p.yaml": tok},
		"profile 文件名非法":     {"profiles/P_1.yaml": tok},
		"profile 扩展名不对":     {"profiles/p.yml": tok},
		"profile 子目录":       {"profiles/sub/p.yaml": tok},
		"未知 profile 字段":     {"profiles/p.yaml": tok + "foo: 1\n"},
		"token 太短":          {"profiles/p.yaml": "token: short\n"},
		"clash 里写 proxies":  {"profiles/p.yaml": tok + "clash: {proxies: []}\n"},
		"节点不存在":             {"profiles/p.yaml": tok + "nodes: [c]\n"},
		"节点不支持正则":           {"profiles/p.yaml": tok + "nodes: ['.*']\n"},
		"节点重复":              {"profiles/p.yaml": tok + "nodes: [a, a]\n"},
		"代理组成员不存在":          {"profiles/p.yaml": tok + "clash: {proxy-groups: [{name: g, type: select, proxies: [c]}]}\n"},
		"规则目标不存在":           {"profiles/p.yaml": tok + "clash: {rules: ['MATCH,proxy']}\n"},
		"rule-provider 不存在": {"profiles/p.yaml": tok + "clash: {rules: ['RULE-SET,cn,DIRECT']}\n"},
		"引用未选中的节点":          {"profiles/p.yaml": tok + "nodes: [a]\nclash: {rules: ['MATCH,b(香港)']}\n"},
		"token 重复":          {"profiles/p.yaml": tok, "profiles/q.yaml": tok},
	}
	for name, files := range cases {
		if _, err := Load(writeDir(t, files)); err == nil {
			t.Errorf("%s: 应该报错", name)
		}
	}
}

func TestLogicRuleSkipped(t *testing.T) {
	dir := writeDir(t, map[string]string{"profiles/p.yaml": `
token: 0123456789abcdef
clash:
  rules:
    - AND,((DOMAIN,a.com),(NETWORK,UDP)),REJECT
    - IP-CIDR,1.2.3.4/32,a,no-resolve
    - MATCH,DIRECT
`})
	if _, err := Load(dir); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprint(t *testing.T) {
	dir := writeDir(t, map[string]string{"profiles/p.yaml": "token: 0123456789abcdef\n"})
	before, err := Fingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "profiles/q.yaml"), []byte("token: fedcba9876543210\n"), 0o600)
	after, _ := Fingerprint(dir)
	if before == after {
		t.Error("新增 profile 后指纹应变化")
	}
}
