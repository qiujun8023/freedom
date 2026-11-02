# Freedom

代理订阅服务：节点只在一份 YAML 里写一次，按设备下发 Clash（mihomo）配置或 V2Ray 分享链接。开发规范和提交规范见 [AGENTS.md](AGENTS.md)。

## 功能

- 每台设备一份 profile，各自的 token、节点范围、代理组和规则
- 按 UA 自动选择格式：mihomo、Clash Meta、FlClash 等拿到完整 Clash 配置，v2rayNG、v2rayN、Shadowrocket、NekoBox 拿到 base64 分享链接
- 节点用 mihomo 的 proxy 写法，分享链接支持 vless（含 REALITY）、vmess、trojan、ss、hysteria2
- 配置文件修改后自动重载；新配置有错时继续使用旧配置，并在日志里报错
- 加载时检查代理组成员、规则目标和 rule-provider 是否存在

## 配置

完整示例见 [config.example.yaml](config.example.yaml)。

```yaml
x-reality: &reality          # x- 开头的顶层字段不读取，用来放 YAML 锚点
  type: vless
  port: 443
  tls: true

nodes:                       # mihomo proxy 写法
  - name: hk-01
    <<: *reality
    server: 203.0.113.10
    uuid: ...

profiles:
  alice-mac:                 # 名字只能用小写字母、数字和短横线
    token: ...               # 至少 16 个字符，各 profile 不能重复
    nodes: [hk-.*]           # 节点名正则（全名匹配），不写表示全部节点
    clash:                   # mihomo 配置，不写 proxies，由 nodes 填入
      proxy-groups: [...]
      rules: [...]
```

- 合并键 `<<: *锚点` 可以复用节点参数和 Clash 基础配置
- `clash` 下的列表里嵌套的列表会被展开，可以写成 `rules: [*mine, *common]` 拼接规则
- 分享链接表达不了的节点（如 wireguard、带插件的 ss）只出现在 Clash 订阅里，`freedom check` 会列出来

## 订阅地址

```
GET /sub/<profile>?token=<token>[&target=clash|v2ray]
```

- 默认按 UA 选择格式，`target` 参数可以强制指定
- token 错误或 profile 不存在一律返回 404
- `GET /healthz` 用于健康检查

## 命令

```bash
freedom                                  # 启动服务
freedom check                            # 校验配置，列出每个 profile 的节点数和跳过的节点
freedom render <profile> [clash|v2ray]   # 打印客户端会拿到的订阅内容
```

| 参数 | 环境变量 | 默认值 |
|---|---|---|
| `-config` | `APP_CONFIG` | `config.yaml`（镜像内为 `/data/config.yaml`） |
| `-listen` | `APP_LISTEN` | `:8000` |

## 部署

镜像由 CI 构建并推送到 `ghcr.io/qiujun8023/freedom`，compose 写法见 [compose.example.yml](compose.example.yml)。

配置放在挂载目录的 `config.yaml`，改完不用重启容器。可以在容器里校验：

```bash
docker exec freedom /freedom check
```
