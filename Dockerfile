FROM logstash

MAINTAINER mian <gopher.mian@outlook.com>

WORKDIR /var/wiseLog

ADD boot.sh .
ADD wiseLog .
ADD templates/ ./templates
ADD entrypoint.sh .

ADD docker-1.12.0.tgz /root
RUN mv /root/docker/docker /bin && rm /root/docker -rf

VOLUME ["/etc/logstash/conf.d"]

ENTRYPOINT ["./boot.sh"]
