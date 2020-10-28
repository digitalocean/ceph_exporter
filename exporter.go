//   Copyright 2016 DigitalOcean
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// Command ceph_exporter provides a Prometheus exporter for a Ceph cluster.
package main

import (
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/digitalocean/ceph_exporter/collectors"
	"github.com/ianschenck/envflag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	defaultCephClusterLabel = "ceph"
	defaultCephConfigPath   = "/etc/ceph/ceph.conf"
	defaultCephUser         = "admin"
	defaultRadosOpTimeout   = 30 * time.Second
)

// This horrible thing is a copy of tcpKeepAliveListener, tweaked to
// specifically check if it hits EMFILE when doing an accept, and if so,
// terminate the process.
const keepAlive time.Duration = 3 * time.Minute

type emfileAwareTcpListener struct {
	*net.TCPListener
	logger *logrus.Logger
}

func (ln emfileAwareTcpListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		if oerr, ok := err.(*net.OpError); ok {
			if serr, ok := oerr.Err.(*os.SyscallError); ok && serr.Err == syscall.EMFILE {
				ln.logger.WithError(err).Fatal("running out of file descriptors")
			}
		}
		// Default return
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(keepAlive)
	return tc, nil
}

// CephExporter wraps all the ceph collectors and provides a single global
// exporter to extracts metrics out of. It also ensures that the collection
// is done in a thread-safe manner, the necessary requirement stated by
// prometheus. It also implements a prometheus.Collector interface in order
// to register it correctly.
type CephExporter struct {
	mu         sync.Mutex
	collectors []prometheus.Collector
	logger     *logrus.Logger
}

// Verify that the exporter implements the interface correctly.
var _ prometheus.Collector = &CephExporter{}

// NewCephExporter creates an instance to CephExporter and returns a reference
// to it. We can choose to enable a collector to extract stats out of by adding
// it to the list of collectors.
func NewCephExporter(conn collectors.Conn, cluster string, config string, rgwMode int, logger *logrus.Logger) *CephExporter {
	c := &CephExporter{
		collectors: []prometheus.Collector{
			collectors.NewClusterUsageCollector(conn, cluster, logger),
			collectors.NewPoolUsageCollector(conn, cluster, logger),
			collectors.NewPoolInfoCollector(conn, cluster, logger),
			collectors.NewClusterHealthCollector(conn, cluster, logger),
			collectors.NewMonitorCollector(conn, cluster, logger),
			collectors.NewOSDCollector(conn, cluster, logger),
		},
		logger: logger,
	}

	switch rgwMode {
	case collectors.RGWModeForeground:
		c.collectors = append(c.collectors,
			collectors.NewRGWCollector(cluster, config, false, logger),
		)

	case collectors.RGWModeBackground:
		c.collectors = append(c.collectors,
			collectors.NewRGWCollector(cluster, config, true, logger),
		)

	case collectors.RGWModeDisabled:
		// nothing to do

	default:
		logger.WithField("rgwMode", rgwMode).Warn("RGW Collector Disabled do to invalid mode")
	}

	return c
}

// Describe sends all the descriptors of the collectors included to
// the provided channel.
func (c *CephExporter) Describe(ch chan<- *prometheus.Desc) {
	for _, cc := range c.collectors {
		cc.Describe(ch)
	}
}

// Collect sends the collected metrics from each of the collectors to
// prometheus. Collect could be called several times concurrently
// and thus its run is protected by a single mutex.
func (c *CephExporter) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cc := range c.collectors {
		cc.Collect(ch)
	}
}

func main() {
	var (
		metricsAddr    = envflag.String("TELEMETRY_ADDR", ":9128", "Host:Port for ceph_exporter's metrics endpoint")
		metricsPath    = envflag.String("TELEMETRY_PATH", "/metrics", "URL path for surfacing metrics to Prometheus")
		exporterConfig = envflag.String("EXPORTER_CONFIG", "/etc/ceph/exporter.yml", "Path to ceph_exporter config")
		rgwMode        = envflag.Int("RGW_MODE", 0, "Enable collection of stats from RGW (0:disabled 1:enabled 2:background)")

		logLevel = envflag.String("LOG_LEVEL", "info", "Logging level. One of: [trace, debug, info, warn, error, fatal, panic]")

		cephCluster        = envflag.String("CEPH_CLUSTER", defaultCephClusterLabel, "Ceph cluster name")
		cephConfig         = envflag.String("CEPH_CONFIG", defaultCephConfigPath, "Path to Ceph config file")
		cephUser           = envflag.String("CEPH_USER", defaultCephUser, "Ceph user to connect to cluster")
		cephRadosOpTimeout = envflag.Duration("CEPH_RADOS_OP_TIMEOUT", defaultRadosOpTimeout, "Ceph rados_osd_op_timeout and rados_mon_op_timeout used to contact cluster (0s means no limit)")
	)

	envflag.Parse()

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	if v, err := logrus.ParseLevel(*logLevel); err != nil {
		logger.WithError(err).Warn("error setting log level")
	} else {
		logger.SetLevel(v)
	}

	clusterConfigs := ([]*ClusterConfig)(nil)

	if fileExists(*exporterConfig) {
		cfg, err := ParseConfig(*exporterConfig)
		if err != nil {
			logger.WithError(err).WithField(
				"file", *exporterConfig,
			).Fatal("error parsing ceph_exporter config file")
		}
		clusterConfigs = cfg.Cluster
	} else {
		clusterConfigs = []*ClusterConfig{
			{
				ClusterLabel: *cephCluster,
				User:         *cephUser,
				ConfigFile:   *cephConfig,
			},
		}
	}

	for _, cluster := range clusterConfigs {
		conn := collectors.NewRadosConn(
			cluster.User,
			cluster.ConfigFile,
			*cephRadosOpTimeout,
			logger)

		prometheus.MustRegister(NewCephExporter(
			conn,
			cluster.ClusterLabel,
			cluster.ConfigFile,
			*rgwMode,
			logger))

		logger.WithField("cluster", cluster.ClusterLabel).Info("exporting cluster")
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Ceph Exporter</title></head>
			<body>
			<h1>Ceph Exporter</h1>
			<p><a href='` + *metricsPath + `'>Metrics</a></p>
			</body>
			</html>`))
	})

	logger.WithField("endpoint", *metricsAddr).Info("starting ceph_exporter listener")

	// Below is essentially http.ListenAndServe(), but using our custom
	// emfileAwareTcpListener that will die if we run out of file descriptors
	ln, err := net.Listen("tcp", *metricsAddr)
	if err != nil {
		logrus.WithError(err).Fatal("error creating listener")
	}

	err = http.Serve(emfileAwareTcpListener{ln.(*net.TCPListener), logger}, nil)
	if err != nil {
		logrus.WithError(err).Fatal("error serving requests")
	}
}
