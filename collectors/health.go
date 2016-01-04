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
	conn Conn

	DegradedPGs   prometheus.Gauge
	UncleanPGs    prometheus.Gauge
	UndersizedPGs prometheus.Gauge
	StalePGs      prometheus.Gauge

	DegradedObjectsCount   prometheus.Gauge
	DegradedObjectsPercent prometheus.Gauge

	OSDsDown prometheus.Gauge
}

// NewClusterHealthCollector creates a new instance of ClusterHealthCollector to collect health
// metrics on.
func NewClusterHealthCollector(conn Conn) *ClusterHealthCollector {
	return &ClusterHealthCollector{
		conn: conn,

		DegradedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "degraded_pgs_count",
				Help:      "No. of PGs in a degraded state",
			},
		),
		UncleanPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "unclean_pgs_count",
				Help:      "No. of PGs in an unclean state",
			},
		),
		UndersizedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "undersized_pgs_count",
				Help:      "No. of undersized PGs in the cluster",
			},
		),
		StalePGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "stale_pgs_count",
				Help:      "No. of stale PGs in the cluster",
			},
		),
		DegradedObjectsCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "degraded_objects_count",
				Help:      "No. of degraded objects across all PGs",
			},
		),
		DegradedObjectsPercent: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "degraded_objects_percent",
				Help:      "Percentage of degraded objects in the cluster",
			},
		),
		OSDsDown: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osds_down_count",
				Help:      "Count of OSDs that are in DOWN state",
			},
		),
	}
}

func (c *ClusterHealthCollector) metricsList() []prometheus.Metric {
	return []prometheus.Metric{
		c.DegradedPGs,
		c.UncleanPGs,
		c.UndersizedPGs,
		c.StalePGs,
		c.DegradedObjectsCount,
		c.DegradedObjectsPercent,
		c.OSDsDown,
	}
}

type cephHealthStats struct {
	Summary []struct {
		Severity string `json:"severity"`
		Summary  string `json:"summary"`
	} `json:"summary"`
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

	if len(stats.Summary) < 1 {
		return nil
	}

	var (
		degradedRegex        = regexp.MustCompile(`([\d]+) pgs degraded`)
		uncleanRegex         = regexp.MustCompile(`([\d]+) pgs stuck unclean`)
		undersizedRegex      = regexp.MustCompile(`([\d]+) pgs undersized`)
		staleRegex           = regexp.MustCompile(`([\d]+) pgs stale`)
		degradedObjectsRegex = regexp.MustCompile(`recovery ([\d]+)/([\d]+) objects degraded \(([\d]+)[^\d]`)
		osdsDownRegex        = regexp.MustCompile(`([\d]+)/([\d]+) in osds are down`)
	)

	for _, s := range stats.Summary {
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
		if len(matched) == 4 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.DegradedObjectsCount.Set(float64(v))

			v, err = strconv.Atoi(matched[3])
			if err != nil {
				return err
			}
			c.DegradedObjectsPercent.Set(float64(v))
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

	return nil
}

func (c *ClusterHealthCollector) cephUsageCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "health",
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
		log.Println("failed collecting metrics:", err)
		return
	}

	for _, metric := range c.metricsList() {
		ch <- metric
	}
}
