# Agent Guide

本文件给 AI 代理使用，说明本仓库内必须遵守的开发和提交规则。面向人的项目说明见 [README.md](README.md)。

## 基本规则

- 默认在 `master` 上开发；提交前确认 `git status --short` 只包含本次任务相关文件。
- 不要回滚用户已有改动；遇到无关脏工作区直接忽略，除非它直接阻塞当前任务。
- 仓库公开：示例和测试只用 `example.com`、`203.0.113.0/24` 这类保留地址和占位 UUID，不要写入真实域名、服务器 IP、节点名或凭据。真实配置只在部署机的挂载目录里。
- 改了配置格式要同步改 `config.example.yaml` 和 README，`internal/config` 的测试会加载示例配置。

## 提交规则

- 提交信息统一使用中文，subject 和 body 都必须写中文。
- 格式：`<type>(<scope>): <中文摘要>`；type 用 `feat`/`fix`/`docs`/`refactor`/`chore`/`build`/`ci`，不随意新增。
- 需要 body 时主题和正文之间保留一个空行，正文用中文条目说明关键改动。
- 需要 body 时用单个 `git commit -m $'...\n\n...'` 传入完整信息，避免交互式编辑器或多次 `-m` 产生多余换行。
- 不要加入 AI 署名、生成标识、协作者 trailer 或工具标记。

## 常用校验

```bash
go test ./...
go run . check -config config.example.yaml
go run . render -config config.example.yaml alice-mac
docker build .
```

CI 会运行测试并构建多架构镜像。
