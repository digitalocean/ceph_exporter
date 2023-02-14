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
	"os/exec"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const rbdPath = "/usr/bin/rbd"

const (
	// RbdMirrorOK denotes the status of the rbd-mirror when healthy.
	RbdMirrorOK = "OK"

	// RbdMirrorWarn denotes the status of rbd-mirror when unhealthy.
	RbdMirrorWarn = "WARNING"

	// RbdMirrorErr denotes the status of rbd-mirror when unhealthy but usually needs
	// manual intervention.
	RbdMirrorErr = "ERROR"
)

type rbdMirrorPoolStatus struct {
	Summary struct {
		Health       string `json:"health"`
		DaemonHealth string `json:"daemon_health"`
		ImageHealth  string `json:"image_health"`
		States       struct {
		} `json:"states"`
	} `json:"summary"`
}

// RbdMirrorStatusCollector displays statistics about each pool in the Ceph cluster.
type RbdMirrorStatusCollector struct {
	config  string
	user    string
	logger  *logrus.Logger
	version *Version

	getRbdMirrorStatus func(config string, user string) ([]byte, error)

	// RbdMirrorStatus shows the overall health status of a rbd-mirror.
	RbdMirrorStatus prometheus.Gauge

	// RbdMirrorDaemonStatus shows the health status of a rbd-mirror daemons.
	RbdMirrorDaemonStatus prometheus.Gauge

	// RbdMirrorImageStatus shows the health status of rbd-mirror images.
	RbdMirrorImageStatus prometheus.Gauge
}

// rbdMirrorStatus get the RBD Mirror Pool Status
var rbdMirrorStatus = func(config string, user string) ([]byte, error) {
	out, err := exec.Command(rbdPath, "-c", config, "--user", user, "mirror", "pool", "status", "--format", "json").Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NewRbdMirrorStatusCollector creates a new RbdMirrorStatusCollector instance
func NewRbdMirrorStatusCollector(exporter *Exporter) *RbdMirrorStatusCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	collector := &RbdMirrorStatusCollector{
		config:  exporter.Config,
		user:    exporter.User,
		logger:  exporter.Logger,
		version: exporter.Version,

		getRbdMirrorStatus: rbdMirrorStatus,

		RbdMirrorStatus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rbd_mirror_pool_status",
				Help:        "Health status of rbd-mirror, can vary only between 3 states (err:2, warn:1, ok:0)",
				ConstLabels: labels,
			},
		),

		RbdMirrorDaemonStatus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rbd_mirror_pool_daemon_status",
				Help:        "Health status of rbd-mirror daemons, can vary only between 3 states (err:2, warn:1, ok:0)",
				ConstLabels: labels,
			},
		),

		RbdMirrorImageStatus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rbd_mirror_pool_image_status",
				Help:        "Health status of rbd-mirror images, can vary only between 3 states (err:2, warn:1, ok:0)",
				ConstLabels: labels,
			},
		),
	}

	return collector
}

func (c *RbdMirrorStatusCollector) metricsList() []prometheus.Metric {

	if c.version.IsAtLeast(Pacific) {
		return []prometheus.Metric{
			c.RbdMirrorStatus,
			c.RbdMirrorDaemonStatus,
			c.RbdMirrorImageStatus,
		}
	} else {
		return []prometheus.Metric{
			c.RbdMirrorStatus,
		}
	}
}

func (c *RbdMirrorStatusCollector) mirrorStatusStringToInt(status string) float64 {
	switch status {
	case RbdMirrorOK:
		return 0
	case RbdMirrorWarn:
		return 1
	case RbdMirrorErr:
		return 2
	default:
		c.logger.Errorf("Unknown rbd-mirror status: %s", status)
		return -1
	}
}

// Describe provides the metrics descriptions to Prometheus
func (c *RbdMirrorStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metricsList() {
		ch <- metric.Desc()
	}
}

// Collect sends all the collected metrics Prometheus.
func (c *RbdMirrorStatusCollector) Collect(ch chan<- prometheus.Metric, version *Version) {
	status, err := rbdMirrorStatus(c.config, c.user)
	if err != nil {
		c.logger.WithError(err).Error("failed to run 'rbd mirror pool status'")
	}
	var rbdStatus rbdMirrorPoolStatus
	if err = json.Unmarshal(status, &rbdStatus); err != nil {
		c.logger.WithError(err).Error("failed to Unmarshal rbd mirror pool status output")
	}

	c.RbdMirrorStatus.Set(c.mirrorStatusStringToInt(rbdStatus.Summary.Health))
	c.version = version

	if c.version.IsAtLeast(Pacific) {
		c.RbdMirrorDaemonStatus.Set(c.mirrorStatusStringToInt(rbdStatus.Summary.DaemonHealth))
		c.RbdMirrorImageStatus.Set(c.mirrorStatusStringToInt(rbdStatus.Summary.ImageHealth))
	}
	for _, metric := range c.metricsList() {
		ch <- metric
	}

}
