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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	cephNamespace = "ceph"
)

// A ClusterUsageCollector is used to gather all the global stats about a
// given ceph cluster. It is sometimes essential to know how fast the cluster
// is growing or shrinking as a whole in order to zero in on the cause. The
// pool specific stats are provided separately.
type ClusterUsageCollector struct {
	conn   Conn
	logger *logrus.Logger

	// GlobalCapacity displays the total storage capacity of the cluster. This
	// information is based on the actual no. of objects that are
	// allocated. It does not take overcommitment into consideration.
	GlobalCapacity prometheus.Gauge

	// UsedCapacity shows the storage under use.
	UsedCapacity prometheus.Gauge

	// AvailableCapacity shows the remaining capacity of the cluster that is
	// left unallocated.
	AvailableCapacity prometheus.Gauge
}

// NewClusterUsageCollector creates and returns the reference to
// ClusterUsageCollector and internally defines each metric that display
// cluster stats.
func NewClusterUsageCollector(conn Conn, cluster string, logger *logrus.Logger) *ClusterUsageCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = cluster

	return &ClusterUsageCollector{
		conn:   conn,
		logger: logger,

		GlobalCapacity: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   cephNamespace,
			Name:        "cluster_capacity_bytes",
			Help:        "Total capacity of the cluster",
			ConstLabels: labels,
		}),
		UsedCapacity: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   cephNamespace,
			Name:        "cluster_used_bytes",
			Help:        "Capacity of the cluster currently in use",
			ConstLabels: labels,
		}),
		AvailableCapacity: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   cephNamespace,
			Name:        "cluster_available_bytes",
			Help:        "Available space within the cluster",
			ConstLabels: labels,
		}),
	}
}

func (c *ClusterUsageCollector) metricsList() []prometheus.Metric {
	return []prometheus.Metric{
		c.GlobalCapacity,
		c.UsedCapacity,
		c.AvailableCapacity,
	}
}

type cephClusterStats struct {
	Stats struct {
		TotalBytes      float64 `json:"total_bytes"`
		TotalUsedBytes  float64 `json:"total_used_bytes"`
		TotalAvailBytes float64 `json:"total_avail_bytes"`
	} `json:"stats"`
}

func (c *ClusterUsageCollector) collect() error {
	cmd := c.cephUsageCommand()
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		c.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	stats := &cephClusterStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	c.GlobalCapacity.Set(stats.Stats.TotalBytes)
	c.UsedCapacity.Set(stats.Stats.TotalUsedBytes)
	c.AvailableCapacity.Set(stats.Stats.TotalAvailBytes)

	return nil
}

func (c *ClusterUsageCollector) cephUsageCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "df",
		"detail": "detail",
		"format": "json",
	})
	if err != nil {
		// panic! because ideally in no world this hard-coded input
		// should fail.
		c.logger.WithError(err).Panic("error marshalling ceph df detail")
	}
	return cmd
}

// Describe sends the descriptors of each metric over to the provided channel.
// The corresponding metric values are sent separately.
func (c *ClusterUsageCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metricsList() {
		ch <- metric.Desc()
	}
}

// Collect sends the metric values for each metric pertaining to the global
// cluster usage over to the provided prometheus Metric channel.
func (c *ClusterUsageCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Debug("collecting cluster usage metrics")
	if err := c.collect(); err != nil {
		c.logger.WithError(err).Error("error collecting cluster usage metrics")
		return
	}

	for _, metric := range c.metricsList() {
		ch <- metric
	}
}
