## 作用

 在国内服务器上面搭建的国内外网络分流方案（路由器上面搭建的方案类似，前提还是需要一个国外服务器或者 Shadowsocks 账号），在国内服务器上面配置好了之后，以后可以在此服务器上面开的 Socks5、HTTP 等代理都国内外自动分流，VPN 的话则需要根据相关情况再配置下 iptables

## 优势

* 通过国内服务器中转一般可以加快访问国外网站的速度，选择合适的服务器线路，速度可以得到很大的提升，并且无丢包，如果国内服务器为 BGP 线路，则优势更大
* 客户端可以使用全局代理方案，一般情况下可以防止运营商网络劫持、WIFI 钓鱼等
* 特殊的组合方式，可以同时实现 防劫持、科学上网、免流等，具体自己研究

## 原理

* DNS 解析使用黑名单机制，gfwlist 里面的域名通过 ss-tunnel 使用 8.8.8.8 解析，其他域名则使用国内 DNS 解析，这样尽可能保证较快的无污染 DNS
* TCP/UDP 流量则根据 iptables 判断是否中国 IP 段，配合 ss-redir 走国外或者国内线路

## 前提

* 安装 ipset (Ubuntu下：apt-get install ipset)
* 安装 [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev)
* 安装 Docker (可选，由于 dnsmasq 的 listen_address 存在莫名的问题，所以这里使用 Docker 处理)

## 第一步：运行 ss-tunnel ss-redir

* 通过 ifconfig 获取 docker0 的 IP，这里假定为 192.168.0.1
* 进入仓库的 shadowsocks 目录，根据情况修改 shadowsocks 配置文件
* 运行：`ss-tunnel -A -u -c config.json -b 192.168.0.1 -l 53 -L 8.8.8.8:53` ，此命令表示将 8.8.8.8 的 53 端口转发的 docker0(192.168.0.1) 的 53 端口上面，其中：
  * -A 表示开启一次性验证
  * -u 表示开启 UDP 转发
  * -c config.json 表示使用上述 shadowsocks 目录下的的 config.json 文件
  * -b 192.168.0.1 表示本地监听 docker0 的 IP （用来给 docker 中运行的 dnsmasq 的 gfwlist 使用）
  * -l 53 表示监听 docker0 的 53 端口
  * -L 8.8.8.8:53 表示远程为 8.8.8.8 的 53 端口

* 运行：`ss-redir -A -u -c config.json -l 1080` 此条命令监听 1080 端口给 iptables 使用

注：可以使用 nohup 之类的保持后台运行，也可以使用 byobu 之类

## 第二步：运行 dnsmasq

* 进入仓库中 dnsmasq 目录，根据实际情况修改 config.json：
```javascript
{
  "update_interval": 21600, // gfwlist 及 adlist 更新间隔（单位：秒）
  "default_dns": [          // 默认 DNS
    "10.202.72.116",
    "10.202.72.118"
  ],
  "default_file": {
    "temp": "/tmp/default.conf",             // 临时文件存放位置，下同
    "target": "/etc/dnsmasq.d/default.conf"  // 目标存放位置，下同
  },
  "gfw_dns": [  // gfwlist中使用的dns，于上方 ss-tunnel 设置的一致即可
    "192.168.0.1#53"
  ],
  "gfw_list": [ // 文件格式如下方示例网址即可
    "https://raw.githubusercontent.com/gfwlist/gfwlist/master/gfwlist.txt"
  ],
  "gfw_list_ex": [  // 额外添加的条目
    "google.com",
    "google.com.hk"
  ],
  "gfw_file": {
    "temp": "/tmp/gfwlist.conf",
    "target": "/etc/dnsmasq.d/gfwlist.conf"
  },
  "ad_list": [
    "https://raw.githubusercontent.com/vokins/yhosts/master/hosts"
  ],
  "ad_list_ex": [
  ],
  "ad_file": {
    "temp": "/tmp/adlist.conf",
    "target": "/etc/dnsmasq.d/adlist.conf"
  }
}
```
* 运行：`docker build .` 制作 docker 镜像，得到镜像 ID ，这里假定为 `abcdefabcdef`
* 运行：`docker run --name dnsmasq -p 53:53 -p 53:53/udp -d abcdefabcdef`
* 如顺利，到此无污染 DNS 已经搭建完毕，可以使用 dig www.google.com @127.0.0.1 测试
* 如测试通过，则进入下一步

## 第三步：导入 iptables

* 进入仓库中 iptables 目录中，修改其中的代理服务器 IP 为你的 shadowsocks 服务端的 IP 地址(这步很重要，如果弄错，可能会导致服务器卡死)
* 运行 `./chnroute.sh`
* 如顺利，则国内外自动分流已完成

## 存在的问题
* 由于在 iptables 将 53 端口的 UDP/TCP 流量都转发到了 127.0.0.1 ，所以在使用 nmap 进行端口扫码等操作是，53 端口正常情况下都是开放的
* 由于只是处理了 UDP/TCP 流量，所以在进行 ping 操作时，正常情况下无法 ping 的服务器可能依旧是无法 ping 通的
