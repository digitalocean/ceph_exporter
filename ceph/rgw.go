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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const rgwGCTimeFormat = "2006-01-02 15:04:05"
const radosgwAdminPath = "/usr/bin/radosgw-admin"
const backgroundCollectInterval = time.Duration(5 * time.Minute)

const (
	RGWModeDisabled   = 0
	RGWModeForeground = 1
	RGWModeBackground = 2
)

type rgwTaskGC struct {
	Tag     string `json:"tag"`
	Time    string `json:"time"`
	Objects []struct {
		Pool     string `json:"pool"`
		OID      string `json:"oid"`
		Key      string `json:"ky"`
		Instance string `json:"instance"`
	} `json:"objs"`
}

// Expires returns the timestamp that this task will expire and become active
func (gc rgwTaskGC) ExpiresAt() time.Time {
	tmp := strings.SplitN(gc.Time, ".", 2)

	last, err := time.Parse(rgwGCTimeFormat, tmp[0])
	if err != nil {
		return time.Now()
	}
	return last
}

// rgwGetGCTaskList get the RGW Garbage Collection task list
func rgwGetGCTaskList(config string, user string) ([]byte, error) {
	var (
		out []byte
		err error
	)

	if out, err = exec.Command(radosgwAdminPath, "-c", config, "--user", user, "gc", "list", "--include-all").Output(); err != nil {
		return nil, err
	}

	return out, nil
}

// RGWCollector collects metrics from the RGW service
type RGWCollector struct {
	config     string
	user       string
	background bool
	logger     *logrus.Logger

	// ActiveTasks reports the number of (expired) RGW GC tasks
	ActiveTasks *prometheus.GaugeVec
	// ActiveObjects reports the total number of RGW GC objects contained in active tasks
	ActiveObjects *prometheus.GaugeVec

	// PendingTasks reports the number of RGW GC tasks queued but not yet expired
	PendingTasks *prometheus.GaugeVec
	// PendingObjects reports the total number of RGW GC objects contained in pending tasks
	PendingObjects *prometheus.GaugeVec

	getRGWGCTaskList func(string, string) ([]byte, error)
}

// NewRGWCollector creates an instance of the RGWCollector and instantiates
// the individual metrics that we can collect from the RGW service
func NewRGWCollector(exporter *Exporter, background bool) *RGWCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	rgw := &RGWCollector{
		config:           exporter.Config,
		user:		  exporter.User,
		background:       background,
		logger:           exporter.Logger,
		getRGWGCTaskList: rgwGetGCTaskList,

		ActiveTasks: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_active_tasks",
				Help:        "RGW GC active task count",
				ConstLabels: labels,
			},
			[]string{},
		),
		ActiveObjects: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_active_objects",
				Help:        "RGW GC active object count",
				ConstLabels: labels,
			},
			[]string{},
		),
		PendingTasks: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_pending_tasks",
				Help:        "RGW GC pending task count",
				ConstLabels: labels,
			},
			[]string{},
		),
		PendingObjects: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_pending_objects",
				Help:        "RGW GC pending object count",
				ConstLabels: labels,
			},
			[]string{},
		),
	}

	if rgw.background {
		// rgw stats need to be collected in the background as this can take a while
		// if we have a large backlog
		go rgw.backgroundCollect()
	}

	return rgw
}

func (r *RGWCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		r.ActiveTasks,
		r.ActiveObjects,
		r.PendingTasks,
		r.PendingObjects,
	}
}

func (r *RGWCollector) backgroundCollect() error {
	for {
		r.logger.WithField("background", r.background).Debug("collecting RGW GC stats")
		err := r.collect()
		if err != nil {
			r.logger.WithField("background", r.background).WithError(err).Error("error collecting RGW GC stats")
		}
		time.Sleep(backgroundCollectInterval)
	}
}

func (r *RGWCollector) collect() error {
	data, err := r.getRGWGCTaskList(r.config, r.user)
	if err != nil {
		return err
	}

	tasks := make([]rgwTaskGC, 0)
	err = json.Unmarshal(data, &tasks)
	if err != nil {
		return err
	}

	activeTaskCount := int(0)
	activeObjectCount := int(0)
	pendingTaskCount := int(0)
	pendingObjectCount := int(0)

	now := time.Now()
	for _, task := range tasks {
		if now.Sub(task.ExpiresAt()) > 0 {
			// timer expired these are active
			activeTaskCount += 1
			activeObjectCount += len(task.Objects)
		} else {
			pendingTaskCount += 1
			pendingObjectCount += len(task.Objects)
		}
	}

	r.ActiveTasks.WithLabelValues().Set(float64(activeTaskCount))
	r.PendingTasks.WithLabelValues().Set(float64(pendingTaskCount))

	r.ActiveObjects.WithLabelValues().Set(float64(activeObjectCount))
	r.PendingObjects.WithLabelValues().Set(float64(pendingObjectCount))

	return nil
}

// Describe sends the descriptors of each RGWCollector related metrics we have defined
// to the provided prometheus channel.
func (r *RGWCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range r.collectorList() {
		metric.Describe(ch)
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (r *RGWCollector) Collect(ch chan<- prometheus.Metric, version *Version) {
	if !r.background {
		r.logger.WithField("background", r.background).Debug("collecting RGW GC stats")
		err := r.collect()
		if err != nil {
			r.logger.WithField("background", r.background).WithError(err).Error("error collecting RGW GC stats")
		}
	}

	for _, metric := range r.collectorList() {
		metric.Collect(ch)
	}
}
