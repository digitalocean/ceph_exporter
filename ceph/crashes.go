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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	statusNames = map[bool]string{true: "new", false: "archived"}
)

// CrashesCollector collects information on how many crash reports are currently open.
// These reports are counted by daemon/client name, and by status (new or archived).
// This is NOT the same as new_crash_reports, that only counts new reports in the past
// two weeks as reported by 'ceph health'.
type CrashesCollector struct {
	conn   Conn
	logger *logrus.Logger

	crashReportsDesc *prometheus.Desc
}

// NewCrashesCollector creates a new CrashesCollector instance
func NewCrashesCollector(exporter *Exporter) *CrashesCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	collector := &CrashesCollector{
		conn:   exporter.Conn,
		logger: exporter.Logger,

		crashReportsDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_crash_reports", cephNamespace),
			"Count of crashes reports per daemon, according to `ceph crash ls`",
			[]string{"entity", "hostname", "status"},
			labels,
		),
	}

	return collector
}

type crashEntry struct {
	entity   string
	hostname string
	isNew    bool
}

type cephCrashLs struct {
	Entity   string `json:"entity_name"`
	Hostname string `json:"utsname_hostname"`
	Archived string `json:"archived"`
}

// getCrashLs runs the 'ceph crash ls' command and process its results
func (c *CrashesCollector) getCrashLs() (map[crashEntry]int, error) {
	crashes := make(map[crashEntry]int)

	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "crash ls",
		"format": "json",
	})
	if err != nil {
		return crashes, err
	}

	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return crashes, err
	}

	var crashData []cephCrashLs
	if err = json.Unmarshal(buf, &crashData); err != nil {
		return crashes, err
	}

	for _, crash := range crashData {
		crashes[crashEntry{crash.Entity, crash.Hostname, len(crash.Archived) == 0}]++
	}

	return crashes, nil
}

// Describe provides the metrics descriptions to Prometheus
func (c *CrashesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.crashReportsDesc
}

// Collect sends all the collected metrics Prometheus.
func (c *CrashesCollector) Collect(ch chan<- prometheus.Metric, version *Version, wg *sync.WaitGroup) {
	defer wg.Done()

	crashes, err := c.getCrashLs()
	if err != nil {
		c.logger.WithError(err).Error("failed to run 'ceph crash ls'")
	}

	for crash, count := range crashes {
		ch <- prometheus.MustNewConstMetric(
			c.crashReportsDesc,
			prometheus.GaugeValue,
			float64(count),
			crash.entity,
			crash.hostname,
			statusNames[crash.isNew],
		)
	}
}
