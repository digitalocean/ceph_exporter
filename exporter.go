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
	"flag"
	"log"
	"net/http"
	"sync"

	"github.com/EmilioD/ceph_exporter/collectors"

	"github.com/ceph/go-ceph/rados"
	"github.com/prometheus/client_golang/prometheus"
)

// CephExporter wraps all the ceph collectors and provides a single global
// exporter to extracts metrics out of. It also ensures that the collection
// is done in a thread-safe manner, the necessary requirement stated by
// prometheus. It also implements a prometheus.Collector interface in order
// to register it correctly.
type CephExporter struct {
	mu         sync.Mutex
	collectors []prometheus.Collector
}

// Verify that the exporter implements the interface correctly.
var _ prometheus.Collector = &CephExporter{}

// NewCephExporter creates an instance to CephExporter and returns a reference
// to it. We can choose to enable a collector to extract stats out of by adding
// it to the list of collectors.
func NewCephExporter(conn *rados.Conn, cluster string) *CephExporter {
	return &CephExporter{
		collectors: []prometheus.Collector{
			collectors.NewClusterUsageCollector(conn, cluster),
			collectors.NewPoolUsageCollector(conn, cluster),
			collectors.NewClusterHealthCollector(conn, cluster),
			collectors.NewMonitorCollector(conn, cluster),
			collectors.NewOSDCollector(conn, cluster),
		},
	}
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
		addr        = flag.String("telemetry.addr", ":9128", "host:port for ceph exporter")
		metricsPath = flag.String("telemetry.path", "/metrics", "URL path for surfacing collected metrics")
		cephConfig  = flag.String("ceph.config", "", "path to ceph config file")
		cephUser    = flag.String("ceph.user", "admin", "Ceph user to connect to cluster.")

		exporterConfig = flag.String("exporter.config", "/etc/ceph/exporter.yml", "Path to ceph exporter config.")
	)
	flag.Parse()

	if fileExists(*exporterConfig) {

		cfg, err := ParseConfig(*exporterConfig)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		for _, cluster := range cfg.Cluster {

			conn, err := rados.NewConnWithUser(cluster.User)
			if err != nil {
				log.Fatalf("cannot create new ceph connection: %s", err)
			}

			err = conn.ReadConfigFile(cluster.ConfigFile)
			if err != nil {
				log.Fatalf("cannot read ceph config file: %s", err)
			}

			if err := conn.Connect(); err != nil {
				log.Fatalf("cannot connect to ceph cluster: %s", err)
			}
			// defer Shutdown to program exit
			defer conn.Shutdown()

			log.Printf("Starting ceph exporter for cluster: %s", cluster.ClusterLabel)
			err = prometheus.Register(NewCephExporter(conn, cluster.ClusterLabel))
			if err != nil {
				log.Fatalf("cannot export cluster: %s error: %v", cluster.ClusterLabel, err)
			}
		}
	} else {
		conn, err := rados.NewConnWithUser(*cephUser)
		if err != nil {
			log.Fatalf("cannot create new ceph connection: %s", err)
		}

		if *cephConfig != "" {
			err = conn.ReadConfigFile(*cephConfig)
		} else {
			err = conn.ReadDefaultConfigFile()
		}
		if err != nil {
			log.Fatalf("cannot read ceph config file: %s", err)
		}

		if err := conn.Connect(); err != nil {
			log.Fatalf("cannot connect to ceph cluster: %s", err)
		}
		defer conn.Shutdown()

		prometheus.MustRegister(NewCephExporter(conn, "ceph"))
	}

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Ceph Exporter</title></head>
			<body>
			<h1>Ceph Exporter</h1>
			<p><a href='` + *metricsPath + `'>Metrics</a></p>
			</body>
			</html>`))
	})

	log.Printf("Starting ceph exporter on %q", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("cannot start ceph exporter: %s", err)
	}
}
