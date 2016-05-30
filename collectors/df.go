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
	"fmt"
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

// DfCollector collects information about the available and used space for
// each pool.
type DfCollector struct {
	// conn holds connection to the Ceph cluster
	conn Conn

	// DfPoolBytesAvailable shows the current bytes available for each pool
	DfPoolBytesAvailable *prometheus.GaugeVec

	// DfPoolBytesUsed shows the current bytes used for each pool
	DfPoolBytesUsed *prometheus.GaugeVec
}

// NewDfCollector creates a new instance of DfCollector to collect df
// metrics on.
func NewDfCollector(conn Conn) *DfCollector {

	return &DfCollector{
		conn: conn,

		DfPoolBytesAvailable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Subsystem: "df",
				Name:      "pool_bytes_available",
				Help:      "Ceph volume available statistics",
			},
			[]string{"pool"},
		),
		DfPoolBytesUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Subsystem: "df",
				Name:      "pool_bytes_used",
				Help:      "Ceph volume usage statistics",
			},
			[]string{"pool"},
		),
	}
}

func (c *DfCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		c.DfPoolBytesAvailable,
		c.DfPoolBytesUsed,
	}
}

type cephDfStats struct {
	Pools []struct {
		Name  string
		Stats struct {
			BytesUsed      json.Number `json:"bytes_used"`
			BytesAvailable json.Number `json:"max_avail"`
		}
	}
}

func (c *DfCollector) collectDfStats() (err error) {
	cmd := c.cephDfCommand()
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

	stats := &cephDfStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	for _, p := range stats.Pools {
		avail, err := p.Stats.BytesAvailable.Float64()
		if err != nil {
			return fmt.Errorf("Cannot convert bytesavailable %s to float64: %s", p.Stats.BytesAvailable, err)
		}
		c.DfPoolBytesAvailable.WithLabelValues(p.Name).Set(avail)
		used, err := p.Stats.BytesUsed.Float64()
		if err != nil {
			return fmt.Errorf("Cannot convert bytesused %s to float64: %s", p.Stats.BytesUsed, err)
		}
		c.DfPoolBytesUsed.WithLabelValues(p.Name).Set(used)
	}
	return err
}

func (c *DfCollector) cephDfCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "df",
		"format": "json",
	})
	if err != nil {
		// panic! because ideally in no world this hard-coded input
		// should fail.
		panic(err)
	}
	return cmd
}

// Describe sends all the descriptions of individual metrics of DfCollector
// to the provided prometheus channel.
func (c *DfCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range c.collectorList() {
		collector.Describe(ch)
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (c *DfCollector) Collect(ch chan<- prometheus.Metric) {
	if err := c.collectDfStats(); err != nil {
		log.Println("failed collecting df stats:", err)
	}

	for _, collector := range c.collectorList() {
		collector.Collect(ch)
	}
}
