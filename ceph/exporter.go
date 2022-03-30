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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"sync"
)

// Exporter wraps all the ceph collectors and provides a single global
// exporter to extracts metrics out of. It also ensures that the collection
// is done in a thread-safe manner, the necessary requirement stated by
// prometheus. It also implements a prometheus.Collector interface in order
// to register it correctly.
type Exporter struct {
	mu      sync.Mutex
	Conn    Conn
	Cluster string
	Config  string
	RgwMode int
	Logger  *logrus.Logger
	Version *Version
}

// NewExporter returns an initialized *Exporter
// We can choose to enable a collector to extract stats out of by adding it to the list of collectors.
func NewExporter(conn Conn, cluster string, config string, rgwMode int, logger *logrus.Logger) *Exporter {
	return &Exporter{
		Conn:    conn,
		Cluster: cluster,
		Config:  config,
		RgwMode: rgwMode,
		Logger:  logger,
	}
}

func (exporter *Exporter) getCollectors() []prometheus.Collector {
	standardCollectors := []prometheus.Collector{
		NewClusterUsageCollector(exporter),
		NewPoolUsageCollector(exporter),
		NewPoolInfoCollector(exporter),
		NewClusterHealthCollector(exporter),
		NewMonitorCollector(exporter),
		NewOSDCollector(exporter),
	}

	switch exporter.RgwMode {
	case RGWModeForeground:
		standardCollectors = append(standardCollectors, NewRGWCollector(exporter, false))
	case RGWModeBackground:
		standardCollectors = append(standardCollectors, NewRGWCollector(exporter, true))
	case RGWModeDisabled:
		// nothing to do
	default:
		exporter.Logger.WithField("RgwMode", exporter.RgwMode).Warn("RGW collector disabled due to invalid mode")
	}

	return standardCollectors
}

func (exporter *Exporter) cephVersionCmd() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "version",
		"format": "json",
	})
	if err != nil {
		exporter.Logger.WithError(err).Panic("failed to marshal ceph version command")
	}

	return cmd
}

func (exporter *Exporter) setCephVersion() error {
	buf, _, err := exporter.Conn.MonCommand(exporter.cephVersionCmd())
	if err != nil {
		return err
	}

	cephVersion := &struct {
		Version string `json:"Version"`
	}{}

	err = json.Unmarshal(buf, cephVersion)
	if err != nil {
		return err
	}

	parsedVersion, err := ParseCephVersion(cephVersion.Version)
	if err != nil {
		exporter.Logger.Info("version " + cephVersion.Version)
		return err
	}

	exporter.Version = parsedVersion

	return nil
}

// Describe sends all the descriptors of the collectors included to
// the provided channel.
func (exporter *Exporter) Describe(ch chan<- *prometheus.Desc) {
	err := exporter.setCephVersion()
	if err != nil {
		exporter.Logger.WithError(err).Error("failed to set ceph Version")
		return
	}

	for _, cc := range exporter.getCollectors() {
		cc.Describe(ch)
	}
}

// Collect sends the collected metrics from each of the collectors to
// prometheus. Collect could be called several times concurrently
// and thus its run is protected by a single mutex.
func (exporter *Exporter) Collect(ch chan<- prometheus.Metric) {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()

	err := exporter.setCephVersion()
	if err != nil {
		exporter.Logger.WithError(err).Error("failed to set ceph Version")
		return
	}

	for _, cc := range exporter.getCollectors() {
		cc.Collect(ch)
	}
}
