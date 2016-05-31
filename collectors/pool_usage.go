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

package collectors

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

// PoolUsageCollector displays statistics about each pool we have created
// in the ceph cluster.
type PoolUsageCollector struct {
	conn Conn

	// UsedBytes tracks the amount of bytes currently allocated for the pool. This
	// does not factor in the overcommitment made for individual images.
	UsedBytes *prometheus.GaugeVec

	// MaxAvail tracks the amount of bytes currently free for the pool,
	// which depends on the replication settings for the pool in question.
	MaxAvail *prometheus.GaugeVec

	// Objects shows the no. of RADOS objects created within the pool.
	Objects *prometheus.GaugeVec

	// ReadIO tracks the read IO calls made for the images within each pool.
	ReadIO *prometheus.CounterVec

	// WriteIO tracks the write IO calls made for the images within each pool.
	WriteIO *prometheus.CounterVec
}

// NewPoolUsageCollector creates a new instance of PoolUsageCollector and returns
// its reference.
func NewPoolUsageCollector(conn Conn) *PoolUsageCollector {
	return &PoolUsageCollector{
		conn: conn,

		UsedBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "pool_used_bytes",
				Help:      "Capacity of the pool that is currently under use",
			},
			[]string{"pool"},
		),
		MaxAvail: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Subsystem: "df",
				Name:      "pool_available_bytes",
				Help:      "Free space for this ceph pool",
			},
			[]string{"pool"},
		),
		Objects: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "pool_objects_total",
				Help:      "Total no. of objects allocated within the pool",
			},
			[]string{"pool"},
		),
		ReadIO: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cephNamespace,
				Name:      "pool_read_total",
				Help:      "Total read i/o calls the pool has been subject to",
			},
			[]string{"pool"},
		),
		WriteIO: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cephNamespace,
				Name:      "pool_write_total",
				Help:      "Total write i/o calls the pool has been subject to",
			},
			[]string{"pool"},
		),
	}
}

func (p *PoolUsageCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		p.UsedBytes,
		p.MaxAvail,
		p.Objects,
		p.ReadIO,
		p.WriteIO,
	}
}

type cephPoolStats struct {
	Pools []struct {
		Name  string `json:"name"`
		ID    int    `json:"id"`
		Stats struct {
			BytesUsed json.Number `json:"bytes_used"`
			MaxAvail  float64     `json:"max_avail"`
			Objects   json.Number `json:"objects"`
			Read      json.Number `json:"rd"`
			Write     json.Number `json:"wr"`
		} `json:"stats"`
	} `json:"pools"`
}

func (p *PoolUsageCollector) collect() error {
	cmd := p.cephUsageCommand()
	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

	stats := &cephPoolStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	if len(stats.Pools) < 1 {
		return errors.New("no pools found in the cluster to report stats on")
	}

	for _, pool := range stats.Pools {
		var bytesUsed, objects, read, write float64

		bytesUsed, err = pool.Stats.BytesUsed.Float64()
		if err != nil {
			log.Printf("[ERROR] cannot extract used bytes for pool %q: %s", pool.Name, err)
		}

		objects, err := pool.Stats.Objects.Float64()
		if err != nil {
			log.Printf("[ERROR] cannot extract total objects for pool %q: %s", pool.Name, err)
		}

		read, err = pool.Stats.Read.Float64()
		if err != nil {
			log.Printf("[ERROR] cannot extract read stats for pool %q: %s", pool.Name, err)
		}

		write, err = pool.Stats.Write.Float64()
		if err != nil {
			log.Printf("[ERROR] cannot extract write stats for pool %q: %s", pool.Name, err)
		}

		p.UsedBytes.WithLabelValues(pool.Name).Set(bytesUsed)
		p.MaxAvail.WithLabelValues(pool.Name).Set(pool.Stats.MaxAvail)
		p.Objects.WithLabelValues(pool.Name).Set(objects)
		p.ReadIO.WithLabelValues(pool.Name).Set(read)
		p.WriteIO.WithLabelValues(pool.Name).Set(write)
	}

	return nil
}

func (p *PoolUsageCollector) cephUsageCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "df",
		"detail": "detail",
		"format": "json",
	})
	if err != nil {
		// panic! because ideally in no world this hard-coded input
		// should fail.
		panic(err)
	}
	return cmd
}

// Describe fulfills the prometheus.Collector's interface and sends the descriptors
// of pool's metrics to the given channel.
func (p *PoolUsageCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range p.collectorList() {
		metric.Describe(ch)
	}
}

// Collect extracts the current values of all the metrics and sends them to the
// prometheus channel.
func (p *PoolUsageCollector) Collect(ch chan<- prometheus.Metric) {
	if err := p.collect(); err != nil {
		log.Println("[ERROR] failed collecting pool usage metrics:", err)
		return
	}

	for _, metric := range p.collectorList() {
		metric.Collect(ch)
	}
}
