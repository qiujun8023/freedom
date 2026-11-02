package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/lmittmann/tint"

	"freedom/internal/config"
	"freedom/internal/server"
)

const usage = `用法:
  freedom [serve]                    启动订阅服务
  freedom check                      校验配置并列出每个 profile 的节点
  freedom render <profile> [target]  打印订阅内容，target 为 clash（默认）或 v2ray

参数:
  -config  配置文件路径，默认读环境变量 APP_CONFIG，再默认 config.yaml
  -listen  监听地址，默认读环境变量 APP_LISTEN，再默认 :8000
`

func main() {
	// 输出不是终端时（如 docker logs）关闭颜色
	fi, _ := os.Stderr.Stat()
	noColor := fi == nil || fi.Mode()&os.ModeCharDevice == 0
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{TimeFormat: time.DateTime, NoColor: noColor})))

	fs := flag.NewFlagSet("freedom", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	configPath := fs.String("config", envOr("APP_CONFIG", "config.yaml"), "")
	listen := fs.String("listen", envOr("APP_LISTEN", ":8000"), "")

	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	fs.Parse(args)

	var err error
	switch cmd {
	case "serve":
		err = serve(*configPath, *listen)
	case "check":
		err = check(*configPath)
	case "render":
		err = render(*configPath, fs.Args())
	default:
		fs.Usage()
		os.Exit(2)
	}
	if err != nil {
		slog.Error(cmd+" 失败", "error", err)
		os.Exit(1)
	}
}

func serve(path, listen string) error {
	store, err := server.NewStore(path)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              listen,
		Handler:           server.Handler(store),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()

	slog.Info("订阅服务已启动", "listen", listen, "config", path, "profiles", len(store.Get().Profiles))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func check(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		p := cfg.Profiles[name]
		fmt.Printf("%s: %d 个节点\n", name, len(p.Nodes))
		for _, s := range p.Skipped {
			fmt.Printf("  分享链接跳过 %s\n", s)
		}
	}
	return nil
}

func render(path string, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return errors.New("用法: freedom render <profile> [clash|v2ray]")
	}
	target := config.TargetClash
	if len(args) == 2 {
		var ok bool
		if target, ok = config.ParseTarget(args[1]); !ok {
			return fmt.Errorf("未知的 target %q", args[1])
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	p, ok := cfg.Profiles[args[0]]
	if !ok {
		return fmt.Errorf("profile %q 不存在", args[0])
	}
	_, err = os.Stdout.Write(p.Outputs[target])
	return err
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
