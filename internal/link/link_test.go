package link

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEncode(t *testing.T) {
	cases := []struct {
		name string
		node map[string]any
		want string
	}{
		{
			name: "vless reality",
			node: map[string]any{
				"name": "hk => jp", "type": "vless", "server": "203.0.113.1", "port": 443,
				"uuid": "u-1", "flow": "xtls-rprx-vision", "tls": true, "servername": "www.example.com",
				"client-fingerprint": "chrome", "network": "tcp",
				"reality-opts": map[string]any{"public-key": "pk", "short-id": "01"},
			},
			want: "vless://u-1@203.0.113.1:443?encryption=none&flow=xtls-rprx-vision&fp=chrome&pbk=pk&security=reality&sid=01&sni=www.example.com&type=tcp#hk%20=%3E%20jp",
		},
		{
			name: "vless ws tls ipv6",
			node: map[string]any{
				"name": "ws", "type": "vless", "server": "2001:db8::1", "port": 8443, "uuid": "u-2",
				"tls": true, "servername": "a.example.com", "network": "ws",
				"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "a.example.com"}},
			},
			want: "vless://u-2@[2001:db8::1]:8443?encryption=none&host=a.example.com&path=%2Fws&security=tls&sni=a.example.com&type=ws#ws",
		},
		{
			name: "vless 无 tls",
			node: map[string]any{"name": "plain", "type": "vless", "server": "h", "port": 80, "uuid": "u"},
			want: "vless://u@h:80?encryption=none&security=none&type=tcp#plain",
		},
		{
			name: "trojan",
			node: map[string]any{"name": "t", "type": "trojan", "server": "h", "port": 443, "password": "p@ss", "sni": "s", "skip-cert-verify": true},
			want: "trojan://p%40ss@h:443?allowInsecure=1&security=tls&sni=s&type=tcp#t",
		},
		{
			name: "ss",
			node: map[string]any{"name": "s", "type": "ss", "server": "h", "port": 8388, "cipher": "aes-128-gcm", "password": "pw"},
			want: "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:pw")) + "@h:8388#s",
		},
		{
			name: "hysteria2",
			node: map[string]any{"name": "h", "type": "hysteria2", "server": "h", "port": 443, "password": "pw", "sni": "s", "obfs": "salamander", "obfs-password": "o"},
			want: "hysteria2://pw@h:443?obfs=salamander&obfs-password=o&sni=s#h",
		},
	}
	for _, c := range cases {
		got, err := Encode(c.node)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestVmess(t *testing.T) {
	got, err := Encode(map[string]any{
		"name": "v", "type": "vmess", "server": "h", "port": 443, "uuid": "u", "alterId": 0,
		"cipher": "auto", "tls": true, "servername": "s", "network": "ws",
		"ws-opts": map[string]any{"path": "/p"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]string
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v["ps"] != "v" || v["net"] != "ws" || v["path"] != "/p" || v["tls"] != "tls" || v["sni"] != "s" || v["aid"] != "0" {
		t.Errorf("vmess = %v", v)
	}
}

func TestUnsupported(t *testing.T) {
	for _, n := range []map[string]any{
		{"name": "x", "type": "wireguard", "server": "h", "port": 1},
		{"name": "x", "type": "ss", "server": "h", "port": 1, "plugin": "obfs"},
		{"name": "x", "type": "vless", "server": "h", "port": 1, "uuid": "u", "network": "kcp"},
	} {
		if _, err := Encode(n); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%v: err = %v", n, err)
		}
	}
}
