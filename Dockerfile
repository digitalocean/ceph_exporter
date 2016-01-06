FROM ubuntu:14.04
MAINTAINER Vaibhav Bhembre <vaibhav@digitalocean.com>

ENV GOPATH /go
ENV APPLOC $GOPATH/src/github.com/digitalocean/ceph_exporter

RUN apt-get update && \
    apt-get install build-essential
    apt-get install -y librados-dev librbd-dev

WORKDIR $APPLOC
RUN go get -d && \
    go build -o /bin/ceph_exporter && \
    rm -rf $GOPATH

ENTRYPOINT ["/bin/ceph_exporter"
