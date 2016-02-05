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
	"log"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// ClusterHealthCollector collects information about the health of an overall cluster.
// It surfaces changes in the ceph parameters unlike data usage that ClusterUsageCollector
// does.
type ClusterHealthCollector struct {
	// conn holds connection to the Ceph cluster
	conn Conn

	// HealthStatus shows the overall health status of a given cluster.
	HealthStatus prometheus.Gauge

	// DegradedPGs shows the no. of PGs that have some of the replicas
	// missing.
	DegradedPGs prometheus.Gauge

	// UncleanPGs shows the no. of PGs that do not have all objects in the PG
	// that are supposed to be in it.
	UncleanPGs prometheus.Gauge

	// UndersizedPGs depicts the no. of PGs that have fewer copies than configured
	// replication level.
	UndersizedPGs prometheus.Gauge

	// StalePGs depicts no. of PGs that are in an unknown state i.e. monitors do not know
	// anything about their latest state since their pg mapping was modified.
	StalePGs prometheus.Gauge

	// DegradedObjectsCount gives the no. of RADOS objects are constitute the degraded PGs.
	DegradedObjectsCount prometheus.Gauge

	// OSDsDown show the no. of OSDs that are in the DOWN state.
	OSDsDown prometheus.Gauge

	// OSDsUp show the no. of OSDs that are in the UP state and are able to serve requests.
	OSDsUp prometheus.Gauge

	// OSDsIn shows the no. of OSDs that are marked as IN in the cluster.
	OSDsIn prometheus.Gauge

	// OSDsNum shows the no. of total OSDs the cluster has.
	OSDsNum prometheus.Gauge

	// RemappedPGs show the no. of PGs that are currently remapped and needs to be moved
	// to newer OSDs.
	RemappedPGs prometheus.Gauge
}

const (
	// CephHealthOK denotes the status of ceph cluster when healthy.
	CephHealthOK = "HEALTH_OK"

	// CephHealthWarn denotes the status of ceph cluster when unhealthy but recovering.
	CephHealthWarn = "HEALTH_WARN"

	// CephHealthErr denotes the status of ceph cluster when unhealthy but usually needs
	// manual intervention.
	CephHealthErr = "HEALTH_ERR"
)

// NewClusterHealthCollector creates a new instance of ClusterHealthCollector to collect health
// metrics on.
func NewClusterHealthCollector(conn Conn) *ClusterHealthCollector {
	return &ClusterHealthCollector{
		conn: conn,

		HealthStatus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "health_status",
				Help:      "Health status of Cluster, can vary only between 3 states (err:2, warn:1, ok:0)",
			},
		),
		DegradedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "degraded_pgs",
				Help:      "No. of PGs in a degraded state",
			},
		),
		UncleanPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "unclean_pgs",
				Help:      "No. of PGs in an unclean state",
			},
		),
		UndersizedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "undersized_pgs",
				Help:      "No. of undersized PGs in the cluster",
			},
		),
		StalePGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "stale_pgs",
				Help:      "No. of stale PGs in the cluster",
			},
		),
		DegradedObjectsCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "degraded_objects",
				Help:      "No. of degraded objects across all PGs",
			},
		),
		OSDsDown: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osds_down",
				Help:      "Count of OSDs that are in DOWN state",
			},
		),
		OSDsUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osds_up",
				Help:      "Count of OSDs that are in UP state",
			},
		),
		OSDsIn: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osds_in",
				Help:      "Count of OSDs that are in IN state and available to serve requests",
			},
		),
		OSDsNum: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osds",
				Help:      "Count of total OSDs in the cluster",
			},
		),
		RemappedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "pgs_remapped",
				Help:      "No. of PGs that are remapped and incurring cluster-wide movement",
			},
		),
	}
}

func (c *ClusterHealthCollector) metricsList() []prometheus.Metric {
	return []prometheus.Metric{
		c.HealthStatus,
		c.DegradedPGs,
		c.UncleanPGs,
		c.UndersizedPGs,
		c.StalePGs,
		c.DegradedObjectsCount,
		c.OSDsDown,
		c.OSDsUp,
		c.OSDsIn,
		c.OSDsNum,
		c.RemappedPGs,
	}
}

type cephHealthStats struct {
	Health struct {
		Summary []struct {
			Severity string `json:"severity"`
			Summary  string `json:"summary"`
		} `json:"summary"`
		OverallStatus string `json:"overall_status"`
	} `json:"health"`
	OSDMap struct {
		OSDMap struct {
			NumOSDs        json.Number `json:"num_osds"`
			NumUpOSDs      json.Number `json:"num_up_osds"`
			NumInOSDs      json.Number `json:"num_in_osds"`
			NumRemappedPGs json.Number `json:"num_remapped_pgs"`
		} `json:"osdmap"`
	} `json:"osdmap"`
}

func (c *ClusterHealthCollector) collect() error {
	cmd := c.cephUsageCommand()
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

	stats := &cephHealthStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	for _, metric := range c.metricsList() {
		if gauge, ok := metric.(prometheus.Gauge); ok {
			gauge.Set(0)
		}
	}

	switch stats.Health.OverallStatus {
	case CephHealthOK:
		c.HealthStatus.Set(0)
	case CephHealthWarn:
		c.HealthStatus.Set(1)
	case CephHealthErr:
		c.HealthStatus.Set(2)
	default:
		c.HealthStatus.Set(2)
	}

	var (
		degradedRegex        = regexp.MustCompile(`([\d]+) pgs degraded`)
		uncleanRegex         = regexp.MustCompile(`([\d]+) pgs stuck unclean`)
		undersizedRegex      = regexp.MustCompile(`([\d]+) pgs undersized`)
		staleRegex           = regexp.MustCompile(`([\d]+) pgs stale`)
		degradedObjectsRegex = regexp.MustCompile(`recovery ([\d]+)/([\d]+) objects degraded`)
		osdsDownRegex        = regexp.MustCompile(`([\d]+)/([\d]+) in osds are down`)
	)

	for _, s := range stats.Health.Summary {
		matched := degradedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.DegradedPGs.Set(float64(v))
		}

		matched = uncleanRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.UncleanPGs.Set(float64(v))
		}

		matched = undersizedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.UndersizedPGs.Set(float64(v))
		}

		matched = staleRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.StalePGs.Set(float64(v))
		}

		matched = degradedObjectsRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 3 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.DegradedObjectsCount.Set(float64(v))
		}

		matched = osdsDownRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 3 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.OSDsDown.Set(float64(v))
		}
	}

	osdsUp, err := stats.OSDMap.OSDMap.NumUpOSDs.Float64()
	if err != nil {
		return err
	}
	c.OSDsUp.Set(osdsUp)

	osdsIn, err := stats.OSDMap.OSDMap.NumInOSDs.Float64()
	if err != nil {
		return err
	}
	c.OSDsIn.Set(osdsIn)

	osdsNum, err := stats.OSDMap.OSDMap.NumOSDs.Float64()
	if err != nil {
		return err
	}
	c.OSDsNum.Set(osdsNum)

	remappedPGs, err := stats.OSDMap.OSDMap.NumRemappedPGs.Float64()
	if err != nil {
		return err
	}
	c.RemappedPGs.Set(remappedPGs)

	return nil
}

func (c *ClusterHealthCollector) cephUsageCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "status",
		"format": "json",
	})
	if err != nil {
		// panic! because ideally in no world this hard-coded input
		// should fail.
		panic(err)
	}
	return cmd
}

// Describe sends all the descriptions of individual metrics of ClusterHealthCollector
// to the provided prometheus channel.
func (c *ClusterHealthCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metricsList() {
		ch <- metric.Desc()
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (c *ClusterHealthCollector) Collect(ch chan<- prometheus.Metric) {
	if err := c.collect(); err != nil {
		log.Println("failed collecting cluster health metrics:", err)
		return
	}

	for _, metric := range c.metricsList() {
		ch <- metric
	}
}
