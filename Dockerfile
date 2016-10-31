FROM centos:7

MAINTAINER mian <huaxiang@wise2c.com>

WORKDIR /var/wise2c

ADD wise-logger .
#ADD template/ ./template

VOLUME ["/tmp/conf.d"]

ENTRYPOINT ["./wise-logger", "-stderrthreshold=INFO"]
