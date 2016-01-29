FROM ubuntu:14.04
MAINTAINER Vaibhav Bhembre <vaibhav@digitalocean.com>

ENV GOROOT /goroot
ENV GOPATH /go
ENV PATH $GOROOT/bin:$PATH
ENV APPLOC $GOPATH/src/github.com/digitalocean/ceph_exporter

ADD . $APPLOC

RUN apt-get update && \
    apt-get install -y build-essential git curl && \
    apt-get install -y librados-dev librbd-dev

RUN \
  mkdir -p /goroot && \
  curl https://storage.googleapis.com/golang/go1.5.2.linux-amd64.tar.gz | tar xvzf - -C /goroot --strip-components=1

WORKDIR $APPLOC
RUN go get -d && \
    go build -o /bin/ceph_exporter

EXPOSE 9128
ENTRYPOINT ["/bin/ceph_exporter"]
