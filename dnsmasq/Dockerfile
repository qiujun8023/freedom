FROM ubuntu:14.04
MAINTAINER Jun.Qiu <i@qiujun.me>

WORKDIR /root

COPY sources.list /etc/apt/sources.list
RUN apt-get update && apt-get install -y python dnsmasq && apt-get clean

COPY gfwlist.py ./
COPY google.list ./

RUN python ./gfwlist.py

EXPOSE 53/tcp
EXPOSE 53/udp

ENTRYPOINT ["dnsmasq", "-7", "/etc/dnsmasq.d"]
CMD ["-S", "114.114.114.114", "-h", "-d"]
