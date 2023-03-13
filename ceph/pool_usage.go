//   Copyright 2022 DigitalOcean
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

package ceph

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// PoolUsageCollector displays statistics about each pool in the Ceph cluster.
type PoolUsageCollector struct {
	conn   Conn
	logger *logrus.Logger

	// UsedBytes tracks the amount of bytes currently allocated for the pool. This
	// does not factor in the overcommitment made for individual images.
	UsedBytes *prometheus.Desc

	// RawUsedBytes tracks the amount of raw bytes currently used for the pool. This
	// factors in the replication factor (size) of the pool.
	RawUsedBytes *prometheus.Desc

	// MaxAvail tracks the amount of bytes currently free for the pool,
	// which depends on the replication settings for the pool in question.
	MaxAvail *prometheus.Desc

	// PercentUsed is the percentage of raw space available to the pool currently in use
	PercentUsed *prometheus.Desc

	// Objects shows the no. of RADOS objects created within the pool.
	Objects *prometheus.Desc

	// DirtyObjects shows the no. of RADOS dirty objects in a cache-tier pool,
	// this doesn't make sense in a regular pool, see:
	// http://lists.ceph.com/pipermail/ceph-users-ceph.com/2015-April/000557.html
	DirtyObjects *prometheus.Desc

	// UnfoundObjects shows the no. of RADOS unfound object within each pool.
	UnfoundObjects *prometheus.Desc

	// ReadIO tracks the read IO calls made for the images within each pool.
	ReadIO *prometheus.Desc

	// Readbytes tracks the read throughput made for the images within each pool.
	ReadBytes *prometheus.Desc

	// WriteIO tracks the write IO calls made for the images within each pool.
	WriteIO *prometheus.Desc

	// WriteBytes tracks the write throughput made for the images within each pool.
	WriteBytes *prometheus.Desc
}

// NewPoolUsageCollector creates a new instance of PoolUsageCollector and returns
// its reference.
func NewPoolUsageCollector(exporter *Exporter) *PoolUsageCollector {
	var (
		subSystem = "pool"
		poolLabel = []string{"pool"}
	)

	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	return &PoolUsageCollector{
		conn:   exporter.Conn,
		logger: exporter.Logger,

		UsedBytes: prometheus.NewDesc(fmt.Sprintf("%s_%s_used_bytes", cephNamespace, subSystem), "Capacity of the pool that is currently under use",
			poolLabel, labels,
		),
		RawUsedBytes: prometheus.NewDesc(fmt.Sprintf("%s_%s_raw_used_bytes", cephNamespace, subSystem), "Raw capacity of the pool that is currently under use, this factors in the size",
			poolLabel, labels,
		),
		MaxAvail: prometheus.NewDesc(fmt.Sprintf("%s_%s_available_bytes", cephNamespace, subSystem), "Free space for the pool",
			poolLabel, labels,
		),
		PercentUsed: prometheus.NewDesc(fmt.Sprintf("%s_%s_percent_used", cephNamespace, subSystem), "Percentage of the capacity available to this pool that is used by this pool",
			poolLabel, labels,
		),
		Objects: prometheus.NewDesc(fmt.Sprintf("%s_%s_objects_total", cephNamespace, subSystem), "Total no. of objects allocated within the pool",
			poolLabel, labels,
		),
		DirtyObjects: prometheus.NewDesc(fmt.Sprintf("%s_%s_dirty_objects_total", cephNamespace, subSystem), "Total no. of dirty objects in a cache-tier pool",
			poolLabel, labels,
		),
		UnfoundObjects: prometheus.NewDesc(fmt.Sprintf("%s_%s_unfound_objects_total", cephNamespace, subSystem), "Total no. of unfound objects for the pool",
			poolLabel, labels,
		),
		ReadIO: prometheus.NewDesc(fmt.Sprintf("%s_%s_read_total", cephNamespace, subSystem), "Total read I/O calls for the pool",
			poolLabel, labels,
		),
		ReadBytes: prometheus.NewDesc(fmt.Sprintf("%s_%s_read_bytes_total", cephNamespace, subSystem), "Total read throughput for the pool",
			poolLabel, labels,
		),
		WriteIO: prometheus.NewDesc(fmt.Sprintf("%s_%s_write_total", cephNamespace, subSystem), "Total write I/O calls for the pool",
			poolLabel, labels,
		),
		WriteBytes: prometheus.NewDesc(fmt.Sprintf("%s_%s_write_bytes_total", cephNamespace, subSystem), "Total write throughput for the pool",
			poolLabel, labels,
		),
	}
}

type cephPoolStats struct {
	Pools []struct {
		Name  string `json:"name"`
		ID    int    `json:"id"`
		Stats struct {
			BytesUsed    float64 `json:"bytes_used"`
			StoredRaw    float64 `json:"stored_raw"`
			Stored       float64 `json:"stored"`
			MaxAvail     float64 `json:"max_avail"`
			PercentUsed  float64 `json:"percent_used"`
			Objects      float64 `json:"objects"`
			DirtyObjects float64 `json:"dirty"`
			ReadIO       float64 `json:"rd"`
			ReadBytes    float64 `json:"rd_bytes"`
			WriteIO      float64 `json:"wr"`
			WriteBytes   float64 `json:"wr_bytes"`
		} `json:"stats"`
	} `json:"pools"`
}

func (p *PoolUsageCollector) collect(ch chan<- prometheus.Metric) error {
	cmd := p.cephUsageCommand()
	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		p.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	stats := &cephPoolStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	for _, pool := range stats.Pools {
		ch <- prometheus.MustNewConstMetric(p.UsedBytes, prometheus.GaugeValue, pool.Stats.Stored, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.RawUsedBytes, prometheus.GaugeValue, math.Max(pool.Stats.StoredRaw, pool.Stats.BytesUsed), pool.Name)
		ch <- prometheus.MustNewConstMetric(p.MaxAvail, prometheus.GaugeValue, pool.Stats.MaxAvail, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.PercentUsed, prometheus.GaugeValue, pool.Stats.PercentUsed, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.Objects, prometheus.GaugeValue, pool.Stats.Objects, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.DirtyObjects, prometheus.GaugeValue, pool.Stats.DirtyObjects, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.ReadIO, prometheus.GaugeValue, pool.Stats.ReadIO, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.ReadBytes, prometheus.GaugeValue, pool.Stats.ReadBytes, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.WriteIO, prometheus.GaugeValue, pool.Stats.WriteIO, pool.Name)
		ch <- prometheus.MustNewConstMetric(p.WriteBytes, prometheus.GaugeValue, pool.Stats.WriteBytes, pool.Name)

		st, err := p.conn.GetPoolStats(pool.Name)
		if err != nil {
			p.logger.WithError(err).WithField(
				"pool", pool.Name,
			).Error("error getting pool stats")

			continue
		}

		ch <- prometheus.MustNewConstMetric(p.UnfoundObjects, prometheus.GaugeValue, float64(st.ObjectsUnfound), pool.Name)
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
		p.logger.WithError(err).Panic("error marshalling ceph df detail")
	}
	return cmd
}

// Describe fulfills the prometheus.Collector's interface and sends the descriptors
// of pool's metrics to the given channel.
func (p *PoolUsageCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.UsedBytes
	ch <- p.RawUsedBytes
	ch <- p.MaxAvail
	ch <- p.PercentUsed
	ch <- p.Objects
	ch <- p.DirtyObjects
	ch <- p.UnfoundObjects
	ch <- p.ReadIO
	ch <- p.ReadBytes
	ch <- p.WriteIO
	ch <- p.WriteBytes
}

// Collect extracts the current values of all the metrics and sends them to the
// prometheus channel.
func (p *PoolUsageCollector) Collect(ch chan<- prometheus.Metric, version *Version, wg *sync.WaitGroup) {
	defer wg.Done()

	p.logger.Debug("collecting pool usage metrics")
	if err := p.collect(ch); err != nil {
		p.logger.WithError(err).Error("error collecting pool usage metrics")
		return
	}
}
