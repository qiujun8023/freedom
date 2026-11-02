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

const testConfig = `
nodes:
  - {name: a, type: vless, server: 1.1.1.1, port: 443, uuid: x}
profiles:
  dev:
    token: 0123456789abcdef
    clash: {rules: ["MATCH,DIRECT"]}
`

func setup(t *testing.T) (string, http.Handler) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(testConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
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

func TestReload(t *testing.T) {
	path, h := setup(t)
	later := time.Now().Add(time.Minute)

	// 写入有错的配置：保留旧配置
	os.WriteFile(path, []byte("nodes: ["), 0o600)
	os.Chtimes(path, later, later)
	if rec := get(h, "/sub/dev?token=0123456789abcdef", ""); rec.Code != http.StatusOK {
		t.Errorf("坏配置不应影响服务: %d", rec.Code)
	}

	// 写入新配置：新 token 生效
	os.WriteFile(path, []byte(strings.Replace(testConfig, "0123456789abcdef", "fedcba9876543210", 1)), 0o600)
	later = later.Add(time.Minute)
	os.Chtimes(path, later, later)
	if rec := get(h, "/sub/dev?token=fedcba9876543210", ""); rec.Code != http.StatusOK {
		t.Errorf("新配置未生效: %d", rec.Code)
	}
	if rec := get(h, "/sub/dev?token=0123456789abcdef", ""); rec.Code != http.StatusNotFound {
		t.Errorf("旧 token 仍可用: %d", rec.Code)
	}
}
