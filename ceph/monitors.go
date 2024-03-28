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

package ceph

import (
	"encoding/json"
	"regexp"

	"github.com/Jeffail/gabs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"golang.org/x/sync/errgroup"
)

// versionRegexp will parse a Nautilus (at least) `ceph versions` key output
// version_tag matches not only 1.2.34, but 1.2.34-123-hash (or anything arbitrarily)
var versionRegexp = regexp.MustCompile(`ceph version (?P<version_tag>\d+\.\d+\.\d+.*) \((?P<sha1>[A-Za-z0-9]{40})\) (?P<release_tag>\w+)`)

// MonitorCollector is used to extract stats related to monitors
// running within Ceph cluster. As we extract information pertaining
// to each monitor instance, there are various vector metrics we
// need to use.
type MonitorCollector struct {
	conn   Conn
	logger *logrus.Logger

	// ClockSkew shows how far the monitor clocks have skewed from each other. This
	// is an important metric because the functioning of Ceph's paxos depends on
	// the clocks being aligned as close to each other as possible.
	ClockSkew *prometheus.GaugeVec

	// Latency displays the time the monitors take to communicate between themselves.
	Latency *prometheus.GaugeVec

	// NodesinQuorum show the size of the working monitor quorum. Any change in this
	// metric can imply a significant issue in the cluster if it is not manually changed.
	NodesinQuorum prometheus.Gauge

	// CephVersions exposes a view of the `ceph versions` command.
	CephVersions *prometheus.GaugeVec

	// CephFeatures exposes a view of the `ceph features` command.
	CephFeatures *prometheus.GaugeVec
}

// NewMonitorCollector creates an instance of the MonitorCollector and instantiates
// the individual metrics that show information about the monitor processes.
func NewMonitorCollector(exporter *Exporter) *MonitorCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	return &MonitorCollector{
		conn:   exporter.Conn,
		logger: exporter.Logger,

		ClockSkew: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_clock_skew_seconds",
				Help:        "Clock skew the monitor node is incurring",
				ConstLabels: labels,
			},
			[]string{"monitor"},
		),
		Latency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_latency_seconds",
				Help:        "Latency the monitor node is incurring",
				ConstLabels: labels,
			},
			[]string{"monitor"},
		),
		NodesinQuorum: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_quorum_count",
				Help:        "The total size of the monitor quorum",
				ConstLabels: labels,
			},
		),
		CephVersions: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "versions",
				Help:        "Counts of current versioned daemons, parsed from `ceph versions`",
				ConstLabels: labels,
			},
			[]string{"daemon", "version_tag", "sha1", "release_name"},
		),
		CephFeatures: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "features",
				Help:        "Counts of current client features, parsed from `ceph features`",
				ConstLabels: labels,
			},
			[]string{"daemon", "release", "features"},
		),
	}
}

func (m *MonitorCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		m.ClockSkew,
		m.Latency,
		m.CephVersions,
		m.CephFeatures,
	}
}

func (m *MonitorCollector) metricsList() []prometheus.Metric {
	return []prometheus.Metric{
		m.NodesinQuorum,
	}
}

type cephTimeSyncStatus struct {
	TimeChecks map[string]struct {
		Health  string      `json:"health"`
		Latency json.Number `json:"latency"`
		Skew    json.Number `json:"skew"`
	} `json:"time_skew_status"`
}

type cephMonitorStats struct {
	Quorum []int `json:"quorum"`
}

// Note that this is a dict with repeating keys in Luminous
type cephFeatureGroup struct {
	Features string `json:"features"`
	Release  string `json:"release"`
	Num      int    `json:"num"`
}

func (m *MonitorCollector) collect() error {
	eg := errgroup.Group{}

	stats := &cephMonitorStats{}
	eg.Go(func() error {
		// Ceph usage
		cmd := m.cephUsageCommand()
		buf, _, err := m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")

			return err
		}

		return json.Unmarshal(buf, stats)
	})

	timeStats := &cephTimeSyncStatus{}
	eg.Go(func() error {
		// Ceph time sync status
		cmd := m.cephTimeSyncStatusCommand()
		buf, _, err := m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")

			return err
		}

		return json.Unmarshal(buf, timeStats)
	})

	var versions map[string]map[string]float64
	eg.Go(func() error {
		// Ceph versions
		cmd, _ := CephVersionsCmd()
		buf, _, err := m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")

			return err
		}

		versions, err = ParseCephVersions(buf)
		return err
	})

	var buf []byte
	eg.Go(func() (err error) {
		// Ceph features
		cmd := m.cephFeaturesCommand()
		buf, _, err = m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")
		}
		return
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	// Like versions, the same with features
	// {"daemon": [ ... ]}
	parsed, err := gabs.ParseJSON(buf)
	if err != nil {
		return err
	}

	parsedMap, err := parsed.ChildrenMap()
	if err != nil {
		return err
	}

	features := make(map[string][]cephFeatureGroup)
	for daemonKey, innerObj := range parsedMap {
		// Read each daemon into an array of feature groups
		featureGroups, err := innerObj.Children()
		if err == gabs.ErrNotObjOrArray {
			continue
		} else if err != nil {
			return err
		}

		for _, grp := range featureGroups {
			var featureGroup cephFeatureGroup
			if _, err := grp.ChildrenMap(); err == gabs.ErrNotObj {
				continue
			}

			if err := json.Unmarshal(grp.Bytes(), &featureGroup); err != nil {
				return err
			}

			features[daemonKey] = append(features[daemonKey], featureGroup)
		}
	}

	// Reset daemon specifc metrics; daemons can leave the cluster
	m.Latency.Reset()
	m.ClockSkew.Reset()
	m.CephVersions.Reset()
	m.CephFeatures.Reset()

	for monNode, tstat := range timeStats.TimeChecks {
		skew, err := tstat.Skew.Float64()
		if err != nil {
			return err
		}
		m.ClockSkew.WithLabelValues(monNode).Set(skew)

		latency, err := tstat.Latency.Float64()
		if err != nil {
			return err
		}
		m.Latency.WithLabelValues(monNode).Set(latency)
	}

	m.NodesinQuorum.Set(float64(len(stats.Quorum)))

	// Ceph versions, one loop for each daemon.
	// In a consistent cluster, there will only be one iteration (and label set) per daemon.
	for daemon, vers := range versions {
		for version, num := range vers {
			// We have a version, which is something like the following, how we want to map it
			// ceph version 12.2.13-30-aabbccdd (c5b1fd521188ddcdedcf6f98ae0e6a02286042f2) luminous (stable)
			//              version_tag          sha1                                      release_tag
			res := versionRegexp.FindStringSubmatch(version)
			if len(res) != 4 {
				m.CephVersions.WithLabelValues(daemon, "unknown", "unknown", "unknown").Set(num)
				continue
			}

			m.CephVersions.WithLabelValues(daemon, res[1], res[2], res[3]).Set(float64(num))
		}
	}

	// Ceph features, generic handling of arbitrary daemons
	for daemon, groups := range features {
		for _, group := range groups {
			m.CephFeatures.WithLabelValues(daemon, group.Release, group.Features).Set(float64(group.Num))
		}
	}

	return nil
}

func (m *MonitorCollector) cephUsageCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "status",
		"format": "json",
	})
	if err != nil {
		m.logger.WithError(err).Panic("error marshalling ceph status")
	}
	return cmd
}

func (m *MonitorCollector) cephTimeSyncStatusCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "time-sync-status",
		"format": "json",
	})
	if err != nil {
		m.logger.WithError(err).Panic("error marshalling ceph time-sync-status")
	}
	return cmd
}

func (m *MonitorCollector) cephFeaturesCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "features",
		"format": "json",
	})
	if err != nil {
		m.logger.WithError(err).Panic("error marshalling ceph features")
	}
	return cmd
}

// Describe sends the descriptors of each Monitor related metric we have defined
// to the channel provided.
func (m *MonitorCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range m.collectorList() {
		metric.Describe(ch)
	}

	for _, metric := range m.metricsList() {
		ch <- metric.Desc()
	}
}

// Collect extracts the given metrics from the Monitors and sends it to the prometheus
// channel.
func (m *MonitorCollector) Collect(ch chan<- prometheus.Metric, version *Version) {
	m.logger.Debug("collecting ceph monitor metrics")
	if err := m.collect(); err != nil {
		m.logger.WithError(err).Error("error collecting ceph monitor metrics")
		return
	}

	for _, metric := range m.collectorList() {
		metric.Collect(ch)
	}

	for _, metric := range m.metricsList() {
		ch <- metric
	}
}
