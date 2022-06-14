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
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	crashLsLineRegex = regexp.MustCompile(`.*_[0-9a-f-]{36}\s+(\S+)\s*(\*)?`)

	statusNames = map[bool]string{true: "new", false: "archived"}
)

// CrashesCollector collects information on how many crash reports are currently open.
// These reports are counted by daemon/client name, and by status (new or archived).
// This is NOT the same as new_crash_reports, that only counts new reports in the past
// two weeks as reported by 'ceph health'.
type CrashesCollector struct {
	conn    Conn
	logger  *logrus.Logger
	version *Version

	// We keep track of which daemons we've seen so that their error count
	// can be reset to zero if the errors get purged.
	knownEntities map[string]bool

	CrashReports prometheus.GaugeVec
}

// NewCrashesCollector creates a new CrashesCollector instance
func NewCrashesCollector(exporter *Exporter) *CrashesCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	collector := &CrashesCollector{
		conn:    exporter.Conn,
		logger:  exporter.Logger,
		version: exporter.Version,

		knownEntities: map[string]bool{},

		CrashReports: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "crash_reports",
				Help:        "Count of crashes reports per daemon, according to `ceph crash ls`",
				ConstLabels: labels,
			},
			[]string{"daemon", "status"},
		),
	}

	return collector
}

type crashEntry struct {
	entity string
	isNew  bool
}

// getCrashLs runs the 'crash ls' command and parses its results
func (c *CrashesCollector) getCrashLs() ([]crashEntry, error) {
	crashes := make([]crashEntry, 0)

	// We parse the plain format because it is quite compact.
	// The JSON output of this command is very verbose and might be too slow
	// to process in an outage storm.
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "crash ls",
		"format": "plain",
	})
	if err != nil {
		return crashes, err
	}

	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return crashes, err
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(buf))
	for scanner.Scan() {
		matched := crashLsLineRegex.FindStringSubmatch(scanner.Text())
		if len(matched) == 3 {
			crashes = append(crashes, crashEntry{matched[1], matched[2] == "*"})
		} else if len(matched) == 2 {
			// Just in case the line-end spaces were stripped
			crashes = append(crashes, crashEntry{matched[1], false})
		}
	}

	return crashes, nil
}

// processCrashLs takes the parsed results from getCrashLs and counts them
// in a map. It also keeps track of which daemons we've see in the past, and
// initializes all counts to zero where needed.
func (c *CrashesCollector) processCrashLs(crashes []crashEntry) map[crashEntry]int {
	crashMap := make(map[crashEntry]int)

	for _, crash := range crashes {
		c.knownEntities[crash.entity] = true
	}
	for entity := range c.knownEntities {
		crashMap[crashEntry{entity, true}] = 0
		crashMap[crashEntry{entity, false}] = 0
	}
	for _, crash := range crashes {
		crashMap[crash]++
	}

	return crashMap
}

// Describe provides the metrics descriptions to Prometheus
func (c *CrashesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.CrashReports.Describe(ch)
}

// Collect sends all the collected metrics Prometheus.
func (c *CrashesCollector) Collect(ch chan<- prometheus.Metric) {
	crashes, err := c.getCrashLs()
	if err != nil {
		c.logger.WithError(err).Error("failed to run 'ceph crash ls'")
	}
	crashMap := c.processCrashLs(crashes)

	for crash, count := range crashMap {
		c.CrashReports.WithLabelValues(crash.entity, statusNames[crash.isNew]).Set(float64(count))
	}

	c.CrashReports.Collect(ch)
}
