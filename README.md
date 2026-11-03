# Freedom

代理订阅服务：节点只写一次，每个订阅一个文件，按客户端下发 Clash（mihomo）配置或 V2Ray 分享链接。开发规范和提交规范见 [AGENTS.md](AGENTS.md)。

## 功能

- 每个订阅一个 profile 文件，各自的 token、节点、代理组和规则
- 按 UA 自动选择格式：mihomo、Clash Meta、FlClash 等拿到完整 Clash 配置，v2rayNG、v2rayN、Shadowrocket、NekoBox 拿到 base64 分享链接
- 节点用 mihomo 的 proxy 写法，分享链接支持 vless（含 REALITY）、vmess、trojan、ss、hysteria2
- 配置目录有修改后自动重载；新配置有错时继续使用旧配置，并在日志里报错
- 加载时检查代理组成员、规则目标和 rule-provider 是否存在

## 配置

配置是一个目录，完整示例见 [example/](example/)：

```
nodes.yaml              # 全部节点
profiles/<名字>.yaml     # 一个订阅一个文件，文件名就是 profile 名
```

`nodes.yaml` 是 mihomo proxy 写法的节点列表，原样输出到 Clash 订阅，并转换成分享链接：

```yaml
- name: hk-01
  type: vless
  server: 203.0.113.10
  port: 443
  uuid: ...
```

profile 文件：

```yaml
token: ...          # 至少 16 个字符，各 profile 不能重复，用 freedom token 生成
nodes:              # 节点名，精确匹配，按 nodes.yaml 中的顺序输出；不写表示全部节点
  - hk-01
  - node: jp-01     # 只在这个订阅里改名显示，clash 里也用新名字引用
    name: 日本
clash:              # 可选，只在输出 Clash 格式时使用：mihomo 配置，不写 proxies
  proxy-groups: [...]
  rules: [...]
```

- profile 文件名只能用小写字母、数字和短横线，扩展名为 `.yaml`；以 `.` 开头的文件忽略，其他不合规的文件报错
- 不写 `clash` 时，Clash 格式只有节点，没有分流规则；v2rayNG 等客户端的订阅本来就只有节点
- 同一文件里可以用 YAML 锚点和合并键复用内容；`clash` 下的列表里嵌套的列表会被展开，可以写成 `rules: [*mine, *common]`
- 分享链接表达不了的节点（如 wireguard、带插件的 ss）只出现在 Clash 订阅里

## 订阅地址

```
GET /sub/<profile>?token=<token>[&target=clash|v2ray][&decode=1]
```

- 默认按 UA 选择格式，`target` 参数可以强制指定
- 返回内容都是纯文本，浏览器打开直接显示；v2ray 格式加 `decode=1` 返回明文分享链接，一行一个
- token 错误或 profile 不存在一律返回 404
- `GET /healthz` 用于健康检查

## 命令

```bash
freedom                                  # 启动服务
freedom check                            # 校验配置，列出每个 profile 的节点数、跳过的节点和没用到的节点
freedom render <profile> [clash|v2ray]   # 打印客户端会拿到的订阅内容
freedom token                            # 生成一个新 token：16 位大小写字母和数字，不含 0 O o 1 l I
```

| 参数 | 环境变量 | 默认值 |
|---|---|---|
| `-dir` | `APP_CONFIG_DIR` | 当前目录（镜像内为 `/data`） |
| `-listen` | `APP_LISTEN` | `:8000` |

## 部署

镜像由 CI 构建并推送到 `ghcr.io/qiujun8023/freedom`，compose 写法见 [compose.example.yml](compose.example.yml)。

配置目录挂载到容器的 `/data`，改完不用重启容器。可以在容器里校验：

```bash
docker exec freedom /freedom check
```
