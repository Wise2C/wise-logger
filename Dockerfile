FROM centos:7

MAINTAINER mian <huaxiang@wise2c.com>

WORKDIR /var/wise2c

ADD wise-logger .
ADD boot.sh .
ADD template/ ./template

VOLUME ["/tmp/conf.d"]

ENTRYPOINT ["bash", "boot.sh"]