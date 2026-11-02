// Package server 提供订阅 HTTP 接口，并在配置目录变化时自动重载。
package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"freedom/internal/config"
)

// Store 持有当前生效的配置，检测到配置目录有变化时重新加载；新配置有错时保留旧配置。
type Store struct {
	dir string

	mu          sync.Mutex
	cfg         *config.Config
	fingerprint string
}

// NewStore 加载配置，首次加载失败直接返回错误。
func NewStore(dir string) (*Store, error) {
	fp, err := config.Fingerprint(dir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, cfg: cfg, fingerprint: fp}, nil
}

// Get 返回当前配置；目录有变化时先尝试重载。
func (s *Store) Get() *config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	fp, err := config.Fingerprint(s.dir)
	if err != nil {
		slog.Error("读取配置目录失败，继续使用旧配置", "error", err)
		return s.cfg
	}
	if fp == s.fingerprint {
		return s.cfg
	}
	// 无论成功与否都记下这次的状态，避免同一个错误每个请求都打一遍日志
	s.fingerprint = fp
	cfg, err := config.Load(s.dir)
	if err != nil {
		slog.Error("配置有误，继续使用旧配置", "error", err)
		return s.cfg
	}
	s.cfg = cfg
	slog.Info("配置已重载", "profiles", len(cfg.Profiles))
	return s.cfg
}

// v2rayAgents 是要返回分享链接的客户端 UA 关键字，其余客户端返回 Clash 配置。
var v2rayAgents = []string{"v2rayng", "v2rayn", "shadowrocket", "nekobox"}

// DetectTarget 按 UA 判断客户端需要的订阅格式。
func DetectTarget(ua string) config.Target {
	ua = strings.ToLower(ua)
	for _, k := range v2rayAgents {
		if strings.Contains(ua, k) {
			return config.TargetV2ray
		}
	}
	return config.TargetClash
}

// Handler 返回 HTTP 路由。
func Handler(store *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /sub/{profile}", func(w http.ResponseWriter, r *http.Request) {
		subscription(store, w, r)
	})
	return mux
}

func subscription(store *Store, w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("profile")
	ua := r.UserAgent()
	log := slog.With("profile", name, "ua", ua, "ip", clientIP(r))

	p := store.Get().Profiles[name]
	token := r.URL.Query().Get("token")
	if p == nil || subtle.ConstantTimeCompare([]byte(token), []byte(p.Token)) != 1 {
		log.Warn("订阅鉴权失败")
		http.NotFound(w, r)
		return
	}

	target := DetectTarget(ua)
	if t := r.URL.Query().Get("target"); t != "" {
		var ok bool
		if target, ok = config.ParseTarget(t); !ok {
			http.Error(w, "unknown target", http.StatusBadRequest)
			return
		}
	}

	h := w.Header()
	switch target {
	case config.TargetClash:
		h.Set("Content-Type", "text/yaml; charset=utf-8")
		h.Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name)+".yaml")
	default:
		h.Set("Content-Type", "text/plain; charset=utf-8")
	}
	h.Set("Profile-Update-Interval", "24")
	h.Set("Cache-Control", "no-store")
	w.Write(p.Outputs[target])
	log.Info("下发订阅", "target", target)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(ip)
	}
	return r.RemoteAddr
}
