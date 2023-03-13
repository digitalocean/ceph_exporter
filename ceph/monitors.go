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
	"errors"
	"regexp"
	"sync"

	"github.com/Jeffail/gabs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
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

	// TotalKBs display the total storage a given monitor node has.
	TotalKBs *prometheus.GaugeVec

	// UsedKBs depict how much of the total storage our monitor process
	// has utilized.
	UsedKBs *prometheus.GaugeVec

	// AvailKBs shows the space left unused.
	AvailKBs *prometheus.GaugeVec

	// PercentAvail shows the amount of unused space as a percentage of total
	// space.
	PercentAvail *prometheus.GaugeVec

	// Store exposes information about internal backing store.
	Store Store

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

// Store displays information about Monitor's FileStore. It is responsible for
// storing all the meta information about the cluster, including monmaps, osdmaps,
// pgmaps, etc. along with logs and other data.
type Store struct {
	// TotalBytes displays the current size of the FileStore.
	TotalBytes *prometheus.GaugeVec

	// SSTBytes shows the amount used by LevelDB's sorted-string tables.
	SSTBytes *prometheus.GaugeVec

	// LogBytes shows the amount used by logs.
	LogBytes *prometheus.GaugeVec

	// MiscBytes shows the amount used by miscellaneous information.
	MiscBytes *prometheus.GaugeVec
}

// NewMonitorCollector creates an instance of the MonitorCollector and instantiates
// the individual metrics that show information about the monitor processes.
func NewMonitorCollector(exporter *Exporter) *MonitorCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	return &MonitorCollector{
		conn:   exporter.Conn,
		logger: exporter.Logger,

		TotalKBs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_capacity_bytes",
				Help:        "Total storage capacity of the monitor node",
				ConstLabels: labels,
			},
			[]string{"monitor"},
		),
		UsedKBs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_used_bytes",
				Help:        "Storage of the monitor node that is currently allocated for use",
				ConstLabels: labels,
			},
			[]string{"monitor"},
		),
		AvailKBs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_avail_bytes",
				Help:        "Total unused storage capacity that the monitor node has left",
				ConstLabels: labels,
			},
			[]string{"monitor"},
		),
		PercentAvail: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "monitor_avail_percent",
				Help:        "Percentage of total unused storage capacity that the monitor node has left",
				ConstLabels: labels,
			},
			[]string{"monitor"},
		),
		Store: Store{
			TotalBytes: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace:   cephNamespace,
					Name:        "monitor_store_capacity_bytes",
					Help:        "Total capacity of the FileStore backing the monitor daemon",
					ConstLabels: labels,
				},
				[]string{"monitor"},
			),
			SSTBytes: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace:   cephNamespace,
					Name:        "monitor_store_sst_bytes",
					Help:        "Capacity of the FileStore used only for raw SSTs",
					ConstLabels: labels,
				},
				[]string{"monitor"},
			),
			LogBytes: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace:   cephNamespace,
					Name:        "monitor_store_log_bytes",
					Help:        "Capacity of the FileStore used only for logging",
					ConstLabels: labels,
				},
				[]string{"monitor"},
			),
			MiscBytes: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace:   cephNamespace,
					Name:        "monitor_store_misc_bytes",
					Help:        "Capacity of the FileStore used only for storing miscellaneous information",
					ConstLabels: labels,
				},
				[]string{"monitor"},
			),
		},
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
		m.TotalKBs,
		m.UsedKBs,
		m.AvailKBs,
		m.PercentAvail,

		m.Store.TotalBytes,
		m.Store.SSTBytes,
		m.Store.LogBytes,
		m.Store.MiscBytes,

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
	Health struct {
		Health struct {
			HealthServices []struct {
				Mons []struct {
					Name         string      `json:"name"`
					KBTotal      json.Number `json:"kb_total"`
					KBUsed       json.Number `json:"kb_used"`
					KBAvail      json.Number `json:"kb_avail"`
					AvailPercent json.Number `json:"avail_percent"`
					StoreStats   struct {
						BytesTotal json.Number `json:"bytes_total"`
						BytesSST   json.Number `json:"bytes_sst"`
						BytesLog   json.Number `json:"bytes_log"`
						BytesMisc  json.Number `json:"bytes_misc"`
					} `json:"store_stats"`
				} `json:"mons"`
			} `json:"health_services"`
		} `json:"health"`
		TimeChecks struct {
			Mons []struct {
				Name    string      `json:"name"`
				Skew    json.Number `json:"skew"`
				Latency json.Number `json:"latency"`
			} `json:"mons"`
		} `json:"timechecks"`
	} `json:"health"`
	Quorum []int `json:"quorum"`
}

// Note that this is a dict with repeating keys in Luminous
type cephFeatureGroup struct {
	Features string `json:"features"`
	Release  string `json:"release"`
	Num      int    `json:"num"`
}

func (m *MonitorCollector) collect() error {
	wg := &sync.WaitGroup{}

	stats := &cephMonitorStats{}
	var statsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Ceph usage
		cmd := m.cephUsageCommand()
		buf, _, err := m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")

			statsErr = err
			return
		}

		if err := json.Unmarshal(buf, stats); err != nil {
			statsErr = err
		}
	}()

	timeStats := &cephTimeSyncStatus{}
	var timeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Ceph time sync status
		cmd := m.cephTimeSyncStatusCommand()
		buf, _, err := m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")

			timeErr = err
			return
		}

		if err := json.Unmarshal(buf, timeStats); err != nil {
			timeErr = err
		}
	}()

	var versionsErr error
	var versions map[string]map[string]float64
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Ceph versions
		cmd, _ := CephVersionsCmd()
		buf, _, err := m.conn.MonCommand(cmd)
		if err != nil {
			m.logger.WithError(err).WithField(
				"args", string(cmd),
			).Error("error executing mon command")

			versionsErr = err
			return
		}

		versions, err = ParseCephVersions(buf)
		if err != nil {
			m.logger.WithError(err).Error("error parsing ceph versions command")
		}
	}()

	var featuresErr error
	var buf []byte
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Ceph features
		cmd := m.cephFeaturesCommand()
		buf, _, featuresErr = m.conn.MonCommand(cmd)
		if featuresErr != nil {
			m.logger.WithError(featuresErr).WithField(
				"args", string(cmd),
			).Error("error executing mon command")
		}
	}()

	wg.Wait()
	if featuresErr != nil || versionsErr != nil || timeErr != nil || statsErr != nil {
		return errors.New("errors occurred in monitors collector")
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
	m.TotalKBs.Reset()
	m.UsedKBs.Reset()
	m.AvailKBs.Reset()
	m.PercentAvail.Reset()
	m.Latency.Reset()
	m.ClockSkew.Reset()
	m.CephVersions.Reset()
	m.CephFeatures.Reset()

	for _, healthService := range stats.Health.Health.HealthServices {
		for _, monstat := range healthService.Mons {
			kbTotal, err := monstat.KBTotal.Float64()
			if err != nil {
				return err
			}
			m.TotalKBs.WithLabelValues(monstat.Name).Set(kbTotal * 1024)

			kbUsed, err := monstat.KBUsed.Float64()
			if err != nil {
				return err
			}
			m.UsedKBs.WithLabelValues(monstat.Name).Set(kbUsed * 1024)

			kbAvail, err := monstat.KBAvail.Float64()
			if err != nil {
				return err
			}
			m.AvailKBs.WithLabelValues(monstat.Name).Set(kbAvail * 1024)

			percentAvail, err := monstat.AvailPercent.Float64()
			if err != nil {
				return err
			}
			m.PercentAvail.WithLabelValues(monstat.Name).Set(percentAvail)

			storeBytes, err := monstat.StoreStats.BytesTotal.Float64()
			if err != nil {
				return err
			}
			m.Store.TotalBytes.WithLabelValues(monstat.Name).Set(storeBytes)

			sstBytes, err := monstat.StoreStats.BytesSST.Float64()
			if err != nil {
				return err
			}
			m.Store.SSTBytes.WithLabelValues(monstat.Name).Set(sstBytes)

			logBytes, err := monstat.StoreStats.BytesLog.Float64()
			if err != nil {
				return err
			}
			m.Store.LogBytes.WithLabelValues(monstat.Name).Set(logBytes)

			miscBytes, err := monstat.StoreStats.BytesMisc.Float64()
			if err != nil {
				return err
			}
			m.Store.MiscBytes.WithLabelValues(monstat.Name).Set(miscBytes)
		}
	}

	for _, monstat := range stats.Health.TimeChecks.Mons {
		skew, err := monstat.Skew.Float64()
		if err != nil {
			return err
		}
		m.ClockSkew.WithLabelValues(monstat.Name).Set(skew)

		latency, err := monstat.Latency.Float64()
		if err != nil {
			return err
		}
		m.Latency.WithLabelValues(monstat.Name).Set(latency)
	}

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
