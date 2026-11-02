package config

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func loadExample(t *testing.T) *Config {
	t.Helper()
	data, err := os.ReadFile("../../config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("示例配置无法加载: %v", err)
	}
	return cfg
}

func TestExample(t *testing.T) {
	cfg := loadExample(t)

	mac := cfg.Profiles["alice-mac"]
	if got := strings.Join(mac.Nodes, "|"); got != "hk-01|jp-01|hk-01 => jp-01" {
		t.Errorf("alice-mac 节点 = %s", got)
	}
	android := cfg.Profiles["alice-android"]
	if got := strings.Join(android.Nodes, "|"); got != "hk-01|hk-01 => jp-01" {
		t.Errorf("alice-android 节点 = %s", got)
	}

	var clash map[string]any
	if err := yaml.Unmarshal(mac.Outputs[TargetClash], &clash); err != nil {
		t.Fatal(err)
	}
	rules := clash["rules"].([]any)
	if len(rules) != 5 || rules[0] != "DOMAIN-SUFFIX,example.org,jp-01" || rules[4] != "MATCH,proxy" {
		t.Errorf("规则没有正确展开: %v", rules)
	}
	proxies := clash["proxies"].([]any)
	first := proxies[0].(map[string]any)
	if first["flow"] != "xtls-rprx-vision" || first["server"] != "203.0.113.10" {
		t.Errorf("锚点没有合并进节点: %v", first)
	}
	if _, ok := clash["dns"]; !ok {
		t.Error("锚点没有合并进 clash 配置")
	}
	if !strings.HasPrefix(string(mac.Outputs[TargetClash]), "mixed-port:") {
		t.Errorf("顶层字段顺序不对:\n%s", mac.Outputs[TargetClash])
	}

	links, err := base64.StdEncoding.DecodeString(string(android.Outputs[TargetV2ray]))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(string(links), "\n")); n != 2 {
		t.Errorf("alice-android 分享链接数 = %d", n)
	}
}

func TestErrors(t *testing.T) {
	const node = `
nodes:
  - {name: a, type: vless, server: 1.1.1.1, port: 443, uuid: x}
`
	cases := map[string]string{
		"未知顶层字段": node + `
foo: 1
profiles: {p: {token: 0123456789abcdef, clash: {}}}`,
		"未知 profile 字段": node + `
profiles: {p: {token: 0123456789abcdef, clash: {}, foo: 1}}`,
		"token 太短": node + `
profiles: {p: {token: short, clash: {}}}`,
		"profile 名非法": node + `
profiles: {P_1: {token: 0123456789abcdef, clash: {}}}`,
		"clash 里写 proxies": node + `
profiles: {p: {token: 0123456789abcdef, clash: {proxies: []}}}`,
		"正则没匹配": node + `
profiles: {p: {token: 0123456789abcdef, nodes: [b], clash: {}}}`,
		"代理组成员不存在": node + `
profiles: {p: {token: 0123456789abcdef, clash: {proxy-groups: [{name: g, type: select, proxies: [b]}]}}}`,
		"规则目标不存在": node + `
profiles: {p: {token: 0123456789abcdef, clash: {rules: ["MATCH,proxy"]}}}`,
		"rule-provider 不存在": node + `
profiles: {p: {token: 0123456789abcdef, clash: {rules: ["RULE-SET,cn,DIRECT"]}}}`,
		"节点未选中却被引用": node + `
  - {name: b, type: vless, server: 1.1.1.1, port: 443, uuid: x}
profiles: {p: {token: 0123456789abcdef, nodes: [a], clash: {rules: ["MATCH,b"]}}}`,
		"token 重复": node + `
profiles:
  p: {token: 0123456789abcdef, clash: {}}
  q: {token: 0123456789abcdef, clash: {}}`,
	}
	for name, data := range cases {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: 应该报错", name)
		}
	}
}

func TestLogicRuleSkipped(t *testing.T) {
	data := `
nodes:
  - {name: a, type: vless, server: 1.1.1.1, port: 443, uuid: x}
profiles:
  p:
    token: 0123456789abcdef
    clash:
      rules:
        - AND,((DOMAIN,a.com),(NETWORK,UDP)),REJECT
        - IP-CIDR,1.2.3.4/32,a,no-resolve
        - MATCH,DIRECT
`
	if _, err := Parse([]byte(data)); err != nil {
		t.Fatal(err)
	}
}
