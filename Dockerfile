FROM ubuntu:16.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends apt-transport-https ca-certificates && \
    echo "deb https://download.ceph.com/debian-jewel xenial main" >> /etc/apt/sources.list && \
    apt-key adv --keyserver ha.pool.sks-keyservers.net --recv E84AC2C0460F3994 && \
    apt-get update && \
    apt-get install -y --no-install-recommends git golang gcc libc6-dev librados-dev librbd-dev

COPY . /go/src/github.com/digitalocean/ceph_exporter

RUN export GOPATH=/go && \
    cd /go/src/github.com/digitalocean/ceph_exporter && \
    go get -v ./...

EXPOSE 9128

ENTRYPOINT ["/go/bin/ceph_exporter"]
