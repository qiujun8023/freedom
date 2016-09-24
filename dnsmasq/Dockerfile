FROM ubuntu:14.04
MAINTAINER Jun.Qiu <i@qiujun.me>

WORKDIR /root

COPY sources.list /etc/apt/sources.list
RUN apt-get update && apt-get install -y python dnsmasq && apt-get clean

COPY gfwlist.py ./
COPY config.json ./
COPY dnsmasq.conf /etc/dnsmasq.conf

EXPOSE 53/tcp
EXPOSE 53/udp

CMD service dnsmasq start && python -u ./gfwlist.py
