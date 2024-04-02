//   Copyright 2024 DigitalOcean
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
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/ianschenck/envflag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/coreweave/ceph_exporter/ceph"
	"github.com/coreweave/ceph_exporter/rados"
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

// Verify that the exporter implements the interface correctly.
var _ prometheus.Collector = &ceph.Exporter{}

func main() {
	var (
		metricsAddr    = envflag.String("TELEMETRY_ADDR", ":9128", "Host:Port for ceph_exporter's metrics endpoint")
		metricsPath    = envflag.String("TELEMETRY_PATH", "/metrics", "URL path for surfacing metrics to Prometheus")
		exporterConfig = envflag.String("EXPORTER_CONFIG", "/etc/ceph/exporter.yml", "Path to ceph_exporter config")
		rgwMode        = envflag.Int("RGW_MODE", 0, "Enable collection of stats from RGW (0:disabled 1:enabled 2:background)")
		mdsMode        = envflag.Int("MDS_MODE", 0, "Enable collection of stats from MDS (0:disabled 1:enabled 2:background)")

		logLevel = envflag.String("LOG_LEVEL", "info", "Logging level. One of: [trace, debug, info, warn, error, fatal, panic]")

		cephCluster        = envflag.String("CEPH_CLUSTER", defaultCephClusterLabel, "Ceph cluster name")
		cephConfig         = envflag.String("CEPH_CONFIG", defaultCephConfigPath, "Path to Ceph config file")
		cephUser           = envflag.String("CEPH_USER", defaultCephUser, "Ceph user to connect to cluster")
		cephRadosOpTimeout = envflag.Duration("CEPH_RADOS_OP_TIMEOUT", defaultRadosOpTimeout, "Ceph rados_osd_op_timeout and rados_mon_op_timeout used to contact cluster (0s means no limit)")

		tlsCertPath = envflag.String("TLS_CERT_FILE_PATH", "", "Path to certificate file for TLS")
		tlsKeyPath  = envflag.String("TLS_KEY_FILE_PATH", "", "Path to key file for TLS")
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
		conn, err := rados.NewRadosConn(
			cluster.User,
			cluster.ConfigFile,
			*cephRadosOpTimeout,
			logger)

		if err != nil {
			logger.WithError(err).WithField("cluster", cluster.ClusterLabel).Fatal("unable to create rados connection for cluster")
		}

		prometheus.MustRegister(ceph.NewExporter(
			conn,
			cluster.ClusterLabel,
			cluster.ConfigFile,
			cluster.User,
			*rgwMode,
			*mdsMode,
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

	if len(*tlsCertPath) != 0 && len(*tlsKeyPath) != 0 {
		server := &http.Server{
			TLSConfig: &tls.Config{
				GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
					caFiles, err := tls.LoadX509KeyPair(*tlsCertPath, *tlsKeyPath)
					if err != nil {
						return nil, err
					}

					return &caFiles, nil
				},
			},
		}

		err = server.ServeTLS(emfileAwareTcpListener{ln.(*net.TCPListener), logger}, "", "")
		if err != nil {
			logrus.WithError(err).Fatal("error serving TLS requests")
		}
	} else {
		err = http.Serve(emfileAwareTcpListener{ln.(*net.TCPListener), logger}, nil)
		if err != nil {
			logrus.WithError(err).Fatal("error serving requests")
		}
	}
}
