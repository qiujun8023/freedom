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
  freedom check                      校验配置，列出每个 profile 的节点数和没用到的节点
  freedom render <profile> [target]  打印订阅内容，target 为 clash（默认）或 v2ray

参数:
  -dir     配置目录，默认读环境变量 APP_CONFIG_DIR，再默认当前目录
  -listen  监听地址，默认读环境变量 APP_LISTEN，再默认 :8000
`

func main() {
	// 输出不是终端时（如 docker logs）关闭颜色
	fi, _ := os.Stderr.Stat()
	noColor := fi == nil || fi.Mode()&os.ModeCharDevice == 0
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{TimeFormat: time.DateTime, NoColor: noColor})))

	fs := flag.NewFlagSet("freedom", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	configDir := fs.String("dir", envOr("APP_CONFIG_DIR", "."), "")
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
		err = serve(*configDir, *listen)
	case "check":
		err = check(*configDir)
	case "render":
		err = render(*configDir, fs.Args())
	default:
		fs.Usage()
		os.Exit(2)
	}
	if err != nil {
		slog.Error(cmd+" 失败", "error", err)
		os.Exit(1)
	}
}

func serve(dir, listen string) error {
	store, err := server.NewStore(dir)
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

	slog.Info("订阅服务已启动", "listen", listen, "dir", dir, "profiles", len(store.Get().Profiles))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func check(dir string) error {
	cfg, err := config.Load(dir)
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
	for _, n := range cfg.Unused {
		fmt.Printf("没有 profile 用到节点 %s\n", n)
	}
	return nil
}

func render(dir string, args []string) error {
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
	cfg, err := config.Load(dir)
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
