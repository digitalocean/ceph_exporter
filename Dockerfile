FROM ubuntu:18.04 as builder
MAINTAINER Vaibhav Bhembre <vaibhav@digitalocean.com>

ARG TEST

ENV GOROOT /goroot
ENV GOPATH /go
ENV PATH $GOROOT/bin:$PATH
ENV APPLOC $GOPATH/src/github.com/digitalocean/ceph_exporter

RUN apt-get update && \
    apt-get install -y apt-transport-https build-essential git curl wget

RUN wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add -
RUN echo "deb https://download.ceph.com/debian-nautilus bionic main" >> /etc/apt/sources.list

RUN apt-get update && \
    apt-get install -y --force-yes librados-dev librbd-dev

RUN \
  mkdir -p /goroot && \
  curl https://storage.googleapis.com/golang/go1.15.3.linux-amd64.tar.gz | tar xvzf - -C /goroot --strip-components=1

ADD . $APPLOC
WORKDIR $APPLOC
RUN go get -d
# The `-tags nautilus` instructs go-ceph to enable additional support nautilus release.
# See https://github.com/ceph/go-ceph#installation
RUN if [ -n "${TEST}" ]; then go test -tags nautilus -v -race -count=1 ./...; fi
RUN go build -tags nautilus -o /bin/ceph_exporter

FROM ubuntu:18.04
MAINTAINER Vaibhav Bhembre <vaibhav@digitalocean.com>

RUN apt-get update && \
    apt-get install -y apt-transport-https curl wget gnupg
RUN wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add -
RUN echo "deb https://download.ceph.com/debian-nautilus bionic main" >> /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --force-yes librados2 librbd1 ceph-common && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/ceph_exporter /bin/ceph_exporter
RUN chmod +x /bin/ceph_exporter

EXPOSE 9128
ENTRYPOINT ["/bin/ceph_exporter"]
