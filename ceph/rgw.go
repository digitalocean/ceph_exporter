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

type rgwReshardOp struct {
	Time          string `json:"time"`
	Tenant        string `json:"tenant"`
	BucketName    string `json:"bucket_name"`
	BucketID      string `json:"bucket_id"`
	NewInstanceID string `json:"new_instance_id"`
	OldNumShards  int    `json:"old_num_shards"`
	NewNumShards  int    `json:"new_num_shards"`
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

// rgwGetReshardList retrieves the list of buckets that are currently being sharded.
func rgwGetReshardList(config string, user string) ([]byte, error) {
	var (
		out []byte
		err error
	)

	if out, err = exec.Command(radosgwAdminPath, "-c", config, "--user", user, "reshard", "list").Output(); err != nil {
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

	// GCActiveTasks reports the number of (expired) RGW GC tasks.
	GCActiveTasks *prometheus.GaugeVec
	// GCActiveObjects reports the total number of RGW GC objects contained in active tasks.
	GCActiveObjects *prometheus.GaugeVec

	// GCPendingTasks reports the number of RGW GC tasks queued but not yet expired.
	GCPendingTasks *prometheus.GaugeVec
	// GCPendingObjects reports the total number of RGW GC objects contained in pending tasks.
	GCPendingObjects *prometheus.GaugeVec

	// ActiveReshards reports the number of active RGW bucket reshard operations.
	ActiveReshards *prometheus.GaugeVec
	// ActiveBucketReshard reports the state of reshard operation for a particular bucket.
	ActiveBucketReshard *prometheus.Desc

	getRGWGCTaskList  func(string, string) ([]byte, error)
	getRGWReshardList func(string, string) ([]byte, error)
}

// NewRGWCollector creates an instance of the RGWCollector and instantiates
// the individual metrics that we can collect from the RGW service
func NewRGWCollector(exporter *Exporter, background bool) *RGWCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	rgw := &RGWCollector{
		config:            exporter.Config,
		user:              exporter.User,
		background:        background,
		logger:            exporter.Logger,
		getRGWGCTaskList:  rgwGetGCTaskList,
		getRGWReshardList: rgwGetReshardList,

		GCActiveTasks: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_active_tasks",
				Help:        "RGW GC active task count",
				ConstLabels: labels,
			},
			[]string{},
		),
		GCActiveObjects: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_active_objects",
				Help:        "RGW GC active object count",
				ConstLabels: labels,
			},
			[]string{},
		),
		GCPendingTasks: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_pending_tasks",
				Help:        "RGW GC pending task count",
				ConstLabels: labels,
			},
			[]string{},
		),
		GCPendingObjects: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_gc_pending_objects",
				Help:        "RGW GC pending object count",
				ConstLabels: labels,
			},
			[]string{},
		),

		ActiveReshards: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "rgw_active_reshards",
				Help:        "RGW active bucket reshard operations",
				ConstLabels: labels,
			},
			[]string{},
		),
		ActiveBucketReshard: prometheus.NewDesc(
			fmt.Sprintf("%s_%s", cephNamespace, "rgw_bucket_reshard"),
			"RGW bucket reshard operation",
			[]string{"bucket"},
			labels,
		),
	}

	return rgw
}

func (r *RGWCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		r.GCActiveTasks,
		r.GCActiveObjects,
		r.GCPendingTasks,
		r.GCPendingObjects,
		r.ActiveReshards,
	}
}

func (r *RGWCollector) descriptorList() []*prometheus.Desc {
	return []*prometheus.Desc{
		r.ActiveBucketReshard,
	}
}

func (r *RGWCollector) backgroundCollect(ch chan<- prometheus.Metric) error {
	for {
		r.logger.WithField("background", r.background).Debug("collecting RGW stats")
		err := r.collect(ch)
		if err != nil {
			r.logger.WithField("background", r.background).WithError(err).Error("error collecting RGW stats")
		}
		time.Sleep(backgroundCollectInterval)
	}
}

func (r *RGWCollector) collect(ch chan<- prometheus.Metric) error {
	data, err := r.getRGWGCTaskList(r.config, r.user)
	if err != nil {
		return fmt.Errorf("failed getting gc task list: %w", err)
	}

	tasks := make([]rgwTaskGC, 0)
	err = json.Unmarshal(data, &tasks)
	if err != nil {
		return fmt.Errorf("failed unmarshalling gc task data: %w", err)
	}

	var (
		gcActiveTaskCount    = int(0)
		gcActiveObjectCount  = int(0)
		gcPendingTaskCount   = int(0)
		gcPendingObjectCount = int(0)
	)

	now := time.Now()
	for _, task := range tasks {
		if now.Sub(task.ExpiresAt()) > 0 {
			// timer expired these are active
			gcActiveTaskCount += 1
			gcActiveObjectCount += len(task.Objects)
		} else {
			gcPendingTaskCount += 1
			gcPendingObjectCount += len(task.Objects)
		}
	}

	r.GCActiveTasks.WithLabelValues().Set(float64(gcActiveTaskCount))
	r.GCPendingTasks.WithLabelValues().Set(float64(gcPendingTaskCount))

	r.GCActiveObjects.WithLabelValues().Set(float64(gcActiveObjectCount))
	r.GCPendingObjects.WithLabelValues().Set(float64(gcPendingObjectCount))

	var (
		activeReshardOps int
	)

	data, err = r.getRGWReshardList(r.config, r.user)
	if err != nil {
		return fmt.Errorf("failed getting bucket reshard list: %w", err)
	}

	ops := make([]rgwReshardOp, 0)
	err = json.Unmarshal(data, &ops)
	if err != nil {
		return fmt.Errorf("failed unmarshalling bucket reshard list: %w", err)
	}

	for _, op := range ops {
		ch <- prometheus.MustNewConstMetric(
			r.ActiveBucketReshard,
			prometheus.GaugeValue,
			float64(1),
			op.BucketName,
		)
	}

	activeReshardOps = len(ops)
	r.ActiveReshards.WithLabelValues().Set(float64(activeReshardOps))
	return nil
}

// Describe sends the descriptors of each RGWCollector related metrics we have defined
// to the provided prometheus channel.
func (r *RGWCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range r.collectorList() {
		metric.Describe(ch)
	}

	for _, metric := range r.descriptorList() {
		ch <- metric
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (r *RGWCollector) Collect(ch chan<- prometheus.Metric, version *Version) {
	if !r.background {
		r.logger.WithField("background", r.background).Debug("collecting RGW stats")
		err := r.collect(ch)
		if err != nil {
			r.logger.WithField("background", r.background).WithError(err).Error("error collecting RGW stats")
		}
	}

	if r.background {
		go r.backgroundCollect(ch)
	}

	for _, metric := range r.collectorList() {
		metric.Collect(ch)
	}
}
