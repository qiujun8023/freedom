#!/bin/sh

# 需要ipset(apt-get install ipset)

# 确保 ROOT 账号运行
if [ "$(id -u)" != "0" ]; then
   echo "请使用ROOT账号运行" 1>&2
   exit 1
fi

# 下载中国IP段
chnroute_url=http://ftp.apnic.net/apnic/stats/apnic/delegated-apnic-latest
[ -r chnroute.txt ] || curl $chnroute_url | grep ipv4 | grep CN | awk -F\| '{ printf("%s/%d\n", $4, 32-log($5)/log(2)) }' > chnroute.txt

iptables -t nat -N SHADOWSOCKS

# Shadowsocks 服务端地址，非常重要！！！
iptables -t nat -A SHADOWSOCKS -d 1.2.3.4 -j RETURN

# 跳过内网网段
iptables -t nat -A SHADOWSOCKS -d 0.0.0.0/8 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 10.0.0.0/8 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 127.0.0.0/8 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 169.254.0.0/16 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 172.16.0.0/12 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 192.168.0.0/16 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 224.0.0.0/4 -j RETURN
iptables -t nat -A SHADOWSOCKS -d 240.0.0.0/4 -j RETURN

# 创建ipset列表
ipset create chnroute hash:net
cat chnroute.txt | xargs -I ip ipset add chnroute ip

# 跳过国内IP段
iptables -t nat -A SHADOWSOCKS -m set --match-set chnroute dst -j RETURN

# 域名解析走本机
iptables -t nat -A SHADOWSOCKS -p tcp --dport 53 -j REDIRECT --to-ports 53
iptables -t nat -A SHADOWSOCKS -p udp --dport 53 -j REDIRECT --to-ports 53

# 其他请求走代理
iptables -t nat -A SHADOWSOCKS -p tcp -j REDIRECT --to-ports 1080
iptables -t nat -A SHADOWSOCKS -p udp -j REDIRECT --to-ports 1080

# iptables -t nat -A PREROUTING -p tcp -j SHADOWSOCKS
iptables -t nat -A OUTPUT -p tcp -j SHADOWSOCKS
iptables -t nat -A OUTPUT -p udp -j SHADOWSOCKS
