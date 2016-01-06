# Ceph Exporter
Prometheus exporter that scrapes meta information about a running ceph cluster. All the information gathered from the cluster is done by interacting with the monitors using an appropriate wrapper over `rados_mon_command()`. Hence, no additional setup is necessary other than having a working ceph cluster.

## Dependencies

You should ideally run this exporter from the client that can talk to
Ceph. Like any other ceph client it needs the following files to run
correctly.

 * `ceph.conf` containing your ceph configuration.
 * `ceph.<user>.keyring` in order to authenticate to your cluster.

Ceph exporter will automatically pick those up if they are present in
any of the [default
locations](http://docs.ceph.com/docs/master/rados/configuration/ceph-conf/#the-configuration-file). Otherwise you will need to provide the configuration manually using `--ceph.config` flag.

We use Ceph's [official Golang client](https://github.com/ceph/go-ceph) to run commands on the cluster.

## Flags

Name | Description | Default
---- | ---- | ----
telemetry.addr | Host:Port pair to run exporter on | `*:9190`
telemetry.path | URL Path for surfacing metrics to prometheus | `/metrics`
ceph.config | Path to ceph configuration file | ""

## Installation

Typical way of installing in Go should work.

```
go install
```

A Makefile is provided in case you find a need for it.

## Contributing

Please refer to the [CONTRIBUTING](CONTRIBUTING.md) guide for more
information on how to submit your changes to this repository.

## Sample view

If you have [promdash](https://github.com/prometheus/promdash) set up you
can generate views like:

![](sample.png)

---

Copyright @ 2016 DigitalOcean™ Inc.
