FROM ubuntu:14.04
MAINTAINER Vaibhav Bhembre <vaibhav@digitalocean.com>

ENV GOROOT /goroot
ENV GOPATH /go
ENV PATH $GOROOT/bin:$PATH
ENV APPLOC $GOPATH/src/github.com/digitalocean/ceph_exporter

RUN apt-get install -y apt-transport-https

RUN echo "deb https://download.ceph.com/debian-jewel trusty main" >> /etc/apt/sources.list

RUN apt-get update && \
    apt-get install -y build-essential git curl

RUN apt-get install -y --force-yes librados-dev librbd-dev

RUN \
  mkdir -p /goroot && \
  curl https://storage.googleapis.com/golang/go1.5.2.linux-amd64.tar.gz | tar xvzf - -C /goroot --strip-components=1

ADD . $APPLOC
WORKDIR $APPLOC
RUN go get -d && \
    go build -o /bin/ceph_exporter

EXPOSE 9128
ENTRYPOINT ["/bin/ceph_exporter"]
