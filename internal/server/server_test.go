package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testNodes   = "- {name: a, type: vless, server: 1.1.1.1, port: 443, uuid: x}\n"
	testProfile = "token: 0123456789abcdef\nclash: {rules: [\"MATCH,DIRECT\"]}\n"
)

// setup 写出配置目录并启动 Handler，返回 profile 文件路径。
func setup(t *testing.T) (string, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "profiles"), 0o755)
	os.WriteFile(filepath.Join(dir, "nodes.yaml"), []byte(testNodes), 0o600)
	path := filepath.Join(dir, "profiles", "dev.yaml")
	if err := os.WriteFile(path, []byte(testProfile), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return path, Handler(store)
}

func get(h http.Handler, target, ua string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("User-Agent", ua)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSubscription(t *testing.T) {
	_, h := setup(t)

	if rec := get(h, "/sub/dev?token=wrong", "clash.meta"); rec.Code != http.StatusNotFound {
		t.Errorf("错误 token 返回 %d", rec.Code)
	}
	if rec := get(h, "/sub/nope?token=0123456789abcdef", "clash.meta"); rec.Code != http.StatusNotFound {
		t.Errorf("不存在的 profile 返回 %d", rec.Code)
	}

	rec := get(h, "/sub/dev?token=0123456789abcdef", "clash.meta")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "proxies:") {
		t.Errorf("clash 订阅: %d %s", rec.Code, rec.Body)
	}

	rec = get(h, "/sub/dev?token=0123456789abcdef", "v2rayNG/1.9.0")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "proxies:") {
		t.Errorf("v2rayNG 应返回分享链接: %s", rec.Body)
	}

	rec = get(h, "/sub/dev?token=0123456789abcdef&target=clash", "Shadowrocket/2000")
	if !strings.Contains(rec.Body.String(), "proxies:") {
		t.Error("target 参数应覆盖 UA 判断")
	}

	if rec := get(h, "/sub/dev?token=0123456789abcdef&target=foo", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("未知 target 返回 %d", rec.Code)
	}
}

func TestDecode(t *testing.T) {
	_, h := setup(t)

	rec := get(h, "/sub/dev?token=0123456789abcdef", "Mozilla/5.0")
	if rec.Header().Get("Content-Disposition") != "" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("应直接显示而不是下载: %v", rec.Header())
	}

	if rec := get(h, "/sub/dev?token=0123456789abcdef&target=v2ray", ""); strings.HasPrefix(rec.Body.String(), "vless://") {
		t.Errorf("不带 decode 应返回 base64: %s", rec.Body)
	}
	if rec := get(h, "/sub/dev?token=0123456789abcdef&target=v2ray&decode=1", ""); !strings.HasPrefix(rec.Body.String(), "vless://") {
		t.Errorf("decode=1 应返回明文分享链接: %s", rec.Body)
	}
	if rec := get(h, "/sub/dev?token=0123456789abcdef&target=clash&decode=1", ""); !strings.Contains(rec.Body.String(), "proxies:") {
		t.Errorf("Clash 格式应忽略 decode: %s", rec.Body)
	}
}

func TestReload(t *testing.T) {
	path, h := setup(t)
	later := time.Now().Add(time.Minute)

	// 写入有错的配置：保留旧配置
	os.WriteFile(path, []byte("token: ["), 0o600)
	os.Chtimes(path, later, later)
	if rec := get(h, "/sub/dev?token=0123456789abcdef", ""); rec.Code != http.StatusOK {
		t.Errorf("坏配置不应影响服务: %d", rec.Code)
	}

	// 写入新配置：新 token 生效
	os.WriteFile(path, []byte(strings.Replace(testProfile, "0123456789abcdef", "fedcba9876543210", 1)), 0o600)
	later = later.Add(time.Minute)
	os.Chtimes(path, later, later)
	if rec := get(h, "/sub/dev?token=fedcba9876543210", ""); rec.Code != http.StatusOK {
		t.Errorf("新配置未生效: %d", rec.Code)
	}
	if rec := get(h, "/sub/dev?token=0123456789abcdef", ""); rec.Code != http.StatusNotFound {
		t.Errorf("旧 token 仍可用: %d", rec.Code)
	}
}
