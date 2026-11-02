// Package link 把 mihomo 格式的节点转换为 v2rayN 风格的分享链接。
package link

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ErrUnsupported 表示该节点无法用分享链接表达。
var ErrUnsupported = errors.New("不支持转换为分享链接")

// Encode 把一个 mihomo proxy 转换为分享链接。
func Encode(n map[string]any) (string, error) {
	p := node(n)
	switch p.str("type") {
	case "vless":
		return vless(p)
	case "trojan":
		return trojan(p)
	case "vmess":
		return vmess(p)
	case "ss":
		return shadowsocks(p)
	case "hysteria2":
		return hysteria2(p)
	}
	return "", fmt.Errorf("%w: 类型 %s", ErrUnsupported, p.str("type"))
}

type node map[string]any

func (n node) str(key string) string {
	switch v := n[key].(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func (n node) bool(key string) bool {
	b, _ := n[key].(bool)
	return b
}

func (n node) sub(key string) node {
	m, _ := n[key].(map[string]any)
	return node(m)
}

func (n node) list(key string) []string {
	var out []string
	if s, ok := n[key].([]any); ok {
		for _, v := range s {
			out = append(out, fmt.Sprint(v))
		}
	}
	return out
}

func (n node) addr() string {
	return net.JoinHostPort(n.str("server"), n.str("port"))
}

func (n node) fragment() string {
	return "#" + (&url.URL{Fragment: n.str("name")}).EscapedFragment()
}

func build(scheme, user string, n node, q url.Values) string {
	u := scheme + "://" + url.User(user).String() + "@" + n.addr()
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u + n.fragment()
}

// transport 写入传输层参数（type、path、host、serviceName）。
func transport(n node, q url.Values) error {
	network := n.str("network")
	if network == "" {
		network = "tcp"
	}
	q.Set("type", network)
	switch network {
	case "tcp":
	case "ws":
		ws := n.sub("ws-opts")
		setIf(q, "path", ws.str("path"))
		setIf(q, "host", ws.sub("headers").str("Host"))
	case "grpc":
		setIf(q, "serviceName", n.sub("grpc-opts").str("grpc-service-name"))
	case "h2":
		h2 := n.sub("h2-opts")
		setIf(q, "path", h2.str("path"))
		setIf(q, "host", strings.Join(h2.list("host"), ","))
	case "httpupgrade":
		hu := n.sub("http-upgrade-opts")
		setIf(q, "path", hu.str("path"))
		setIf(q, "host", hu.sub("headers").str("Host"))
	default:
		return fmt.Errorf("%w: 传输层 %s", ErrUnsupported, network)
	}
	return nil
}

// security 写入 TLS / REALITY 参数。
func security(n node, q url.Values, tls bool) {
	reality := n.sub("reality-opts")
	switch {
	case reality != nil:
		q.Set("security", "reality")
		setIf(q, "pbk", reality.str("public-key"))
		setIf(q, "sid", reality.str("short-id"))
	case tls:
		q.Set("security", "tls")
	default:
		q.Set("security", "none")
		return
	}
	sni := n.str("servername")
	if sni == "" {
		sni = n.str("sni")
	}
	setIf(q, "sni", sni)
	setIf(q, "fp", n.str("client-fingerprint"))
	setIf(q, "alpn", strings.Join(n.list("alpn"), ","))
	if n.bool("skip-cert-verify") {
		q.Set("allowInsecure", "1")
	}
}

func vless(n node) (string, error) {
	q := url.Values{}
	enc := n.str("encryption")
	if enc == "" {
		enc = "none"
	}
	q.Set("encryption", enc)
	setIf(q, "flow", n.str("flow"))
	if err := transport(n, q); err != nil {
		return "", err
	}
	security(n, q, n.bool("tls"))
	return build("vless", n.str("uuid"), n, q), nil
}

func trojan(n node) (string, error) {
	q := url.Values{}
	if err := transport(n, q); err != nil {
		return "", err
	}
	security(n, q, true)
	return build("trojan", n.str("password"), n, q), nil
}

func vmess(n node) (string, error) {
	q := url.Values{}
	if err := transport(n, q); err != nil {
		return "", err
	}
	security(n, q, n.bool("tls"))
	if q.Get("security") == "reality" {
		return "", fmt.Errorf("%w: vmess 不支持 reality", ErrUnsupported)
	}
	tls := ""
	if q.Get("security") == "tls" {
		tls = "tls"
	}
	cipher := n.str("cipher")
	if cipher == "" {
		cipher = "auto"
	}
	aid := n.str("alterId")
	if aid == "" {
		aid = "0"
	}
	body, err := json.Marshal(map[string]string{
		"v": "2", "ps": n.str("name"), "add": n.str("server"), "port": n.str("port"),
		"id": n.str("uuid"), "aid": aid, "scy": cipher, "net": q.Get("type"), "type": "none",
		"host": q.Get("host"), "path": q.Get("path") + q.Get("serviceName"), "tls": tls,
		"sni": q.Get("sni"), "fp": q.Get("fp"), "alpn": q.Get("alpn"),
	})
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(body), nil
}

func shadowsocks(n node) (string, error) {
	if n["plugin"] != nil {
		return "", fmt.Errorf("%w: ss 插件", ErrUnsupported)
	}
	user := base64.RawURLEncoding.EncodeToString([]byte(n.str("cipher") + ":" + n.str("password")))
	return "ss://" + user + "@" + n.addr() + n.fragment(), nil
}

func hysteria2(n node) (string, error) {
	q := url.Values{}
	setIf(q, "sni", n.str("sni"))
	setIf(q, "obfs", n.str("obfs"))
	setIf(q, "obfs-password", n.str("obfs-password"))
	setIf(q, "alpn", strings.Join(n.list("alpn"), ","))
	if n.bool("skip-cert-verify") {
		q.Set("insecure", "1")
	}
	return build("hysteria2", n.str("password"), n, q), nil
}

func setIf(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}
