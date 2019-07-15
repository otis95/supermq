# FROM buildpack-deps:stretch-scm
FROM centos:7
# FROM golang:1
# FROM alpine:latest
MAINTAINER Jerry Bo <bcw104@126.com>

# FROM alpine:latest
LABEL Description="SuperMQ"

ENV DEMO false

# RUN adduser  -D  iotopo
RUN mkdir -p /supermq /super/log  /super/config
    # chown -R iotopo:iotopo /iotopo

COPY docker-entrypoint.sh /
COPY supermq /usr/supermq
# COPY storage /iotopo/storage_default

RUN chown -R root:root /docker-entrypoint.sh && \
    chown -R root:root /supermq && \
    chmod +x /docker-entrypoint.sh && \
    chmod +x /supermq/supermq
# USER iotopo
VOLUME ["supermq/config"]

WORKDIR /supermq
EXPOSE 1883
ENTRYPOINT ["/docker-entrypoint.sh"]
# ENTRYPOINT ["/iotopo/iot-site"]
# CMD ["cp", "/iotopo/storage_default/*", "/iotopo/storage"]
CMD ["/supermq/supermq", "-c", "/supermq/config/appserver.conf"]
