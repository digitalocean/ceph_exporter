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
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	osdLabelFormat = "osd.%v"
)

const (
	scrubStateIdle          = 0
	scrubStateScrubbing     = 1
	scrubStateDeepScrubbing = 2
)

// OSDCollector displays statistics about OSD in the Ceph cluster.
// An important aspect of monitoring OSDs is to ensure that when the cluster is
// up and running that all OSDs that are in the cluster are up and running, too
type OSDCollector struct {
	conn   Conn
	logger *logrus.Logger

	// osdScrubCache holds the cache of previous PG scrubs
	osdScrubCache map[int]int

	// osdLabelsCache holds a cache of osd labels
	osdLabelsCache map[int64]*cephOSDLabel

	// oldestInactivePGMap keeps track of how long we've known
	// a PG to not have an active state in it.
	oldestInactivePGMap map[string]time.Time

	// pgDumpBrief holds the content of PG dump brief
	pgDumpBrief cephPGDumpBrief

	// CrushWeight is a persistent setting, and it affects how CRUSH assigns data to OSDs.
	// It displays the CRUSH weight for the OSD
	CrushWeight *prometheus.GaugeVec

	// Depth displays the OSD's level of hierarchy in the CRUSH map
	Depth *prometheus.GaugeVec

	// Reweight sets an override weight on the OSD.
	// It displays value within 0 to 1.
	Reweight *prometheus.GaugeVec

	// Bytes displays the total bytes available in the OSD
	Bytes *prometheus.GaugeVec

	// UsedBytes displays the total used bytes in the OSD
	UsedBytes *prometheus.GaugeVec

	// AvailBytes displays the total available bytes in the OSD
	AvailBytes *prometheus.GaugeVec

	// Utilization displays current utilization of the OSD
	Utilization *prometheus.GaugeVec

	// Variance displays current variance of the OSD from the standard utilization
	Variance *prometheus.GaugeVec

	// Pgs displays total number of placement groups in the OSD.
	// Available in Ceph Jewel version.
	Pgs *prometheus.GaugeVec

	// PgUpmapItemsTotal displays the total number of items in the pg-upmap exception table.
	PgUpmapItemsTotal prometheus.Gauge

	// CommitLatency displays in seconds how long it takes for an operation to be applied to disk
	CommitLatency *prometheus.GaugeVec

	// ApplyLatency displays in seconds how long it takes to get applied to the backing filesystem
	ApplyLatency *prometheus.GaugeVec

	// OSDIn displays the In state of the OSD
	OSDIn *prometheus.GaugeVec

	// OSDUp displays the Up state of the OSD
	OSDUp *prometheus.GaugeVec

	// OSDFullRatio displays current full_ratio of OSD
	OSDFullRatio prometheus.Gauge

	// OSDFullRatio displays current backfillfull_ratio of OSD
	OSDBackfillFullRatio prometheus.Gauge

	// OSDNearFullRatio displays current nearfull_ratio of OSD
	OSDNearFullRatio prometheus.Gauge

	// OSDFull flags if an OSD is full
	OSDFull *prometheus.GaugeVec

	// OSDNearfull flags if an OSD is near full
	OSDNearFull *prometheus.GaugeVec

	// OSDBackfillFull flags if an OSD is backfill full
	OSDBackfillFull *prometheus.GaugeVec

	// OSDDownDesc displays OSDs present in the cluster in "down" state
	OSDDownDesc *prometheus.Desc

	// TotalBytes displays total bytes in all OSDs
	TotalBytes prometheus.Gauge

	// TotalUsedBytes displays total used bytes in all OSDs
	TotalUsedBytes prometheus.Gauge

	// TotalAvailBytes displays total available bytes in all OSDs
	TotalAvailBytes prometheus.Gauge

	// AverageUtil displays average utilization in all OSDs
	AverageUtil prometheus.Gauge

	// ScrubbingStateDesc depicts if an OSD is being scrubbed
	// labeled by OSD
	ScrubbingStateDesc *prometheus.Desc

	// PGObjectsRecoveredDesc displays total number of objects recovered in a PG
	PGObjectsRecoveredDesc *prometheus.Desc

	// OSDObjectsBackfilled displays average number of objects backfilled in an OSD
	OSDObjectsBackfilled *prometheus.CounterVec

	// OldestInactivePG gives us the amount of time that the oldest inactive PG
	// has been inactive for.  This is useful to discern between rolling peering
	// (such as when issuing a bunch of upmaps or weight changes) and a single PG
	// stuck peering, for example.
	OldestInactivePG prometheus.Gauge
}

// NewOSDCollector creates an instance of the OSDCollector and instantiates the
// individual metrics that show information about the OSD.
func NewOSDCollector(exporter *Exporter) *OSDCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster
	osdLabels := []string{"osd", "device_class", "host", "rack", "root"}

	return &OSDCollector{
		conn:   exporter.Conn,
		logger: exporter.Logger,

		osdScrubCache:       make(map[int]int),
		osdLabelsCache:      make(map[int64]*cephOSDLabel),
		oldestInactivePGMap: make(map[string]time.Time),

		CrushWeight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_crush_weight",
				Help:        "OSD Crush Weight",
				ConstLabels: labels,
			},
			osdLabels,
		),

		Depth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_depth",
				Help:        "OSD Depth",
				ConstLabels: labels,
			},
			osdLabels,
		),

		Reweight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_reweight",
				Help:        "OSD Reweight",
				ConstLabels: labels,
			},
			osdLabels,
		),

		Bytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_bytes",
				Help:        "OSD Total Bytes",
				ConstLabels: labels,
			},
			osdLabels,
		),

		UsedBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_used_bytes",
				Help:        "OSD Used Storage in Bytes",
				ConstLabels: labels,
			},
			osdLabels,
		),

		AvailBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_avail_bytes",
				Help:        "OSD Available Storage in Bytes",
				ConstLabels: labels,
			},
			osdLabels,
		),

		Utilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_utilization",
				Help:        "OSD Utilization",
				ConstLabels: labels,
			},
			osdLabels,
		),

		Variance: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_variance",
				Help:        "OSD Variance",
				ConstLabels: labels,
			},
			osdLabels,
		),

		Pgs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_pgs",
				Help:        "OSD Placement Group Count",
				ConstLabels: labels,
			},
			osdLabels,
		),

		PgUpmapItemsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_pg_upmap_items_total",
				Help:        "OSD PG-Upmap Exception Table Entry Count",
				ConstLabels: labels,
			},
		),

		TotalBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_total_bytes",
				Help:        "OSD Total Storage Bytes",
				ConstLabels: labels,
			},
		),
		TotalUsedBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_total_used_bytes",
				Help:        "OSD Total Used Storage Bytes",
				ConstLabels: labels,
			},
		),

		TotalAvailBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_total_avail_bytes",
				Help:        "OSD Total Available Storage Bytes ",
				ConstLabels: labels,
			},
		),

		AverageUtil: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_average_utilization",
				Help:        "OSD Average Utilization",
				ConstLabels: labels,
			},
		),

		CommitLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_perf_commit_latency_seconds",
				Help:        "OSD Perf Commit Latency",
				ConstLabels: labels,
			},
			osdLabels,
		),

		ApplyLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_perf_apply_latency_seconds",
				Help:        "OSD Perf Apply Latency",
				ConstLabels: labels,
			},
			osdLabels,
		),

		OSDIn: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_in",
				Help:        "OSD In Status",
				ConstLabels: labels,
			},
			osdLabels,
		),

		OSDUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_up",
				Help:        "OSD Up Status",
				ConstLabels: labels,
			},
			osdLabels,
		),

		OSDFullRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_full_ratio",
				Help:        "OSD Full Ratio Value",
				ConstLabels: labels,
			},
		),

		OSDNearFullRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_near_full_ratio",
				Help:        "OSD Near Full Ratio Value",
				ConstLabels: labels,
			},
		),

		OSDBackfillFullRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_backfill_full_ratio",
				Help:        "OSD Backfill Full Ratio Value",
				ConstLabels: labels,
			},
		),

		OSDFull: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_full",
				Help:        "OSD Full Status",
				ConstLabels: labels,
			},
			osdLabels,
		),

		OSDNearFull: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_near_full",
				Help:        "OSD Near Full Status",
				ConstLabels: labels,
			},
			osdLabels,
		),

		OSDBackfillFull: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_backfill_full",
				Help:        "OSD Backfill Full Status",
				ConstLabels: labels,
			},
			osdLabels,
		),

		OSDDownDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_osd_down", cephNamespace),
			"Number of OSDs down in the cluster",
			append([]string{"status"}, osdLabels...),
			labels,
		),

		ScrubbingStateDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_osd_scrub_state", cephNamespace),
			"State of OSDs involved in a scrub",
			osdLabels,
			labels,
		),

		PGObjectsRecoveredDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_pg_objects_recovered", cephNamespace),
			"Number of objects recovered in a PG",
			[]string{"pgid"},
			labels,
		),

		OSDObjectsBackfilled: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   cephNamespace,
				Name:        "osd_objects_backfilled",
				Help:        "Average number of objects backfilled in an OSD",
				ConstLabels: labels,
			},
			append([]string{"pgid"}, osdLabels...),
		),

		OldestInactivePG: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "pg_oldest_inactive",
				Help:        "The amount of time in seconds that the oldest PG has been inactive for",
				ConstLabels: labels,
			},
		),
	}
}

func (o *OSDCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		o.CrushWeight,
		o.Depth,
		o.Reweight,
		o.Bytes,
		o.UsedBytes,
		o.AvailBytes,
		o.Utilization,
		o.Variance,
		o.Pgs,
		o.PgUpmapItemsTotal,
		o.TotalBytes,
		o.TotalUsedBytes,
		o.TotalAvailBytes,
		o.AverageUtil,
		o.CommitLatency,
		o.ApplyLatency,
		o.OSDIn,
		o.OSDUp,
		o.OSDFullRatio,
		o.OSDNearFullRatio,
		o.OSDBackfillFullRatio,
		o.OSDFull,
		o.OSDNearFull,
		o.OSDBackfillFull,
		o.OSDObjectsBackfilled,
		o.OldestInactivePG,
	}
}

type cephOSDDF struct {
	OSDNodes []struct {
		Name        string      `json:"name"`
		CrushWeight json.Number `json:"crush_weight"`
		Depth       json.Number `json:"depth"`
		Reweight    json.Number `json:"reweight"`
		KB          json.Number `json:"kb"`
		UsedKB      json.Number `json:"kb_used"`
		AvailKB     json.Number `json:"kb_avail"`
		Utilization json.Number `json:"utilization"`
		Variance    json.Number `json:"var"`
		Pgs         json.Number `json:"pgs"`
	} `json:"nodes"`

	Summary struct {
		TotalKB      json.Number `json:"total_kb"`
		TotalUsedKB  json.Number `json:"total_kb_used"`
		TotalAvailKB json.Number `json:"total_kb_avail"`
		AverageUtil  json.Number `json:"average_utilization"`
	} `json:"summary"`
}

type cephPerfStat struct {
	PerfInfo []struct {
		ID    json.Number `json:"id"`
		Stats struct {
			CommitLatency json.Number `json:"commit_latency_ms"`
			ApplyLatency  json.Number `json:"apply_latency_ms"`
		} `json:"perf_stats"`
	} `json:"osd_perf_infos"`
}

type CephOSDPerfStat struct {
	cephPerfStat `json:"osdstats"`
}

type cephOSDDump struct {
	OSDs []struct {
		OSD   json.Number `json:"osd"`
		Up    json.Number `json:"up"`
		In    json.Number `json:"in"`
		State []string    `json:"state"`
	} `json:"osds"`

	PgUpmapItems []struct {
		PgID     string `json:"pgid"`
		Mappings []struct {
			From int `json:"from"`
			To   int `json:"to"`
		} `json:"mappings"`
	} `json:"pg_upmap_items"`

	FullRatio         json.Number `json:"full_ratio"`
	NearFullRatio     json.Number `json:"nearfull_ratio"`
	BackfillFullRatio json.Number `json:"backfillfull_ratio"`
}

type cephOSDTree struct {
	Nodes []struct {
		ID          int64   `json:"id"`
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		Status      string  `json:"status"`
		Class       string  `json:"device_class"`
		CrushWeight float64 `json:"crush_weight"`
		Children    []int64 `json:"children"`
	} `json:"nodes"`
	Stray []struct {
		ID          int64   `json:"id"`
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		Status      string  `json:"status"`
		CrushWeight float64 `json:"crush_weight"`
		Children    []int   `json:"children"`
	} `json:"stray"`
}

type osdNode struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type cephOSDTreeDown struct {
	Nodes []osdNode `json:"nodes"`
	Stray []osdNode `json:"stray"`
}

type cephPGDumpBrief struct {
	PGStats []struct {
		PGID          string `json:"pgid"`
		ActingPrimary int64  `json:"acting_primary"`
		Acting        []int  `json:"acting"`
		State         string `json:"state"`
	} `json:"pg_stats"`
}

type cephOSDLabel struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	DeviceClass string  `json:"device_class"`
	CrushWeight float64 `json:"crush_weight"`
	Root        string  `json:"root"`
	Rack        string  `json:"rack"`
	Host        string  `json:"host"`
	parent      int64   // parent id when building tables
}

func (o *OSDCollector) collectOSDDF() error {
	args := o.cephOSDDFCommand()
	buf, _, err := o.conn.MgrCommand(args)
	if err != nil {
		o.logger.WithError(err).WithField(
			"args", string(bytes.Join(args, []byte(","))),
		).Error("error executing mgr command")

		return err
	}

	// Workaround for Ceph Jewel after 10.2.5 produces invalid json when OSD is out
	buf = bytes.Replace(buf, []byte("-nan"), []byte("0"), -1)

	osdDF := &cephOSDDF{}
	if err := json.Unmarshal(buf, osdDF); err != nil {
		return err
	}

	for _, node := range osdDF.OSDNodes {
		lb := o.getOSDLabelFromName(node.Name)

		crushWeight, err := node.CrushWeight.Float64()
		if err != nil {
			return err
		}

		o.CrushWeight.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(crushWeight)
		depth, err := node.Depth.Float64()
		if err != nil {

			return err
		}

		o.Depth.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(depth)

		reweight, err := node.Reweight.Float64()
		if err != nil {
			return err
		}

		o.Reweight.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(reweight)

		osdKB, err := node.KB.Float64()
		if err != nil {
			return nil
		}

		o.Bytes.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(osdKB * 1024)

		usedKB, err := node.UsedKB.Float64()
		if err != nil {
			return err
		}

		o.UsedBytes.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(usedKB * 1024)

		availKB, err := node.AvailKB.Float64()
		if err != nil {
			return err
		}

		o.AvailBytes.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(availKB * 1024)

		util, err := node.Utilization.Float64()
		if err != nil {
			return err
		}

		o.Utilization.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(util)

		variance, err := node.Variance.Float64()
		if err != nil {
			return err
		}

		o.Variance.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(variance)

		pgs, err := node.Pgs.Float64()
		if err != nil {
			continue
		}

		o.Pgs.WithLabelValues(node.Name, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(pgs)

	}

	totalKB, err := osdDF.Summary.TotalKB.Float64()
	if err != nil {
		return err
	}

	o.TotalBytes.Set(totalKB * 1024)

	totalUsedKB, err := osdDF.Summary.TotalUsedKB.Float64()
	if err != nil {
		return err
	}

	o.TotalUsedBytes.Set(totalUsedKB * 1024)

	totalAvailKB, err := osdDF.Summary.TotalAvailKB.Float64()
	if err != nil {
		return err
	}

	o.TotalAvailBytes.Set(totalAvailKB * 1024)

	averageUtil, err := osdDF.Summary.AverageUtil.Float64()
	if err != nil {
		return err
	}

	o.AverageUtil.Set(averageUtil)

	return nil

}

func (o *OSDCollector) collectOSDPerf() error {
	args := o.cephOSDPerfCommand()
	buf, _, err := o.conn.MgrCommand(args)
	if err != nil {
		o.logger.WithError(err).WithField(
			"args", string(bytes.Join(args, []byte(","))),
		).Error("error executing mon command")

		return err
	}

	osdPerf := &CephOSDPerfStat{}
	if err := json.Unmarshal(buf, osdPerf); err != nil {
		return err
	}

	for _, perfStat := range osdPerf.PerfInfo {
		osdID, err := perfStat.ID.Int64()
		if err != nil {
			return err
		}
		osdName := fmt.Sprintf(osdLabelFormat, osdID)

		lb := o.getOSDLabelFromID(osdID)

		commitLatency, err := perfStat.Stats.CommitLatency.Float64()
		if err != nil {
			return err
		}
		o.CommitLatency.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(commitLatency / 1000)

		applyLatency, err := perfStat.Stats.ApplyLatency.Float64()
		if err != nil {
			return err
		}
		o.ApplyLatency.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(applyLatency / 1000)
	}

	return nil
}

func buildOSDLabels(data []byte) (map[int64]*cephOSDLabel, error) {
	nodeList := &cephOSDTree{}
	if err := json.Unmarshal(data, nodeList); err != nil {
		return nil, err
	}

	nodeMap := make(map[int64]*cephOSDLabel)
	for _, node := range nodeList.Nodes {
		label := cephOSDLabel{
			ID:          node.ID,
			Name:        node.Name,
			Type:        node.Type,
			Status:      node.Status,
			DeviceClass: node.Class,
			CrushWeight: node.CrushWeight,
			parent:      math.MaxInt64,
		}
		nodeMap[node.ID] = &label
	}
	// now that we built a lookup table, fill in the parents
	for _, node := range nodeList.Nodes {
		for _, child := range node.Children {
			if label, ok := nodeMap[child]; ok {
				label.parent = node.ID
			}
		}
	}

	var findParent func(from *cephOSDLabel, kind string) (*cephOSDLabel, bool)
	findParent = func(from *cephOSDLabel, kind string) (*cephOSDLabel, bool) {
		if parent, ok := nodeMap[from.parent]; ok {
			if parent.Type == kind {
				return parent, true
			}
			return findParent(parent, kind)
		}
		return nil, false
	}

	// Now that we have parents filled in walk our map, and build a map of just osds.
	for k := range nodeMap {
		osdLabel := nodeMap[k]
		if host, ok := findParent(osdLabel, "host"); ok {
			osdLabel.Host = host.Name
		}
		if rack, ok := findParent(osdLabel, "rack"); ok {
			osdLabel.Rack = rack.Name
		}
		if root, ok := findParent(osdLabel, "root"); ok {
			osdLabel.Root = root.Name
		}
	}

	for k := range nodeMap {
		osdLabel := nodeMap[k]
		if osdLabel.Type != "osd" {
			delete(nodeMap, k)
		}
	}
	return nodeMap, nil
}

func (o *OSDCollector) buildOSDLabelCache() error {
	cmd := o.cephOSDTreeCommand()
	data, _, err := o.conn.MonCommand(cmd)
	if err != nil {
		o.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	cache, err := buildOSDLabels(data)
	if err != nil {
		return err
	}
	o.osdLabelsCache = cache
	return nil
}

func (o *OSDCollector) getOSDLabelFromID(id int64) *cephOSDLabel {
	if label, ok := o.osdLabelsCache[id]; ok {
		return label
	}
	return &cephOSDLabel{}
}

func (o *OSDCollector) getOSDLabelFromName(osdid string) *cephOSDLabel {
	var id int64
	c, err := fmt.Sscanf(osdid, "osd.%d", &id)
	if err != nil || c != 1 {
		return &cephOSDLabel{}
	}

	return o.getOSDLabelFromID(id)
}

func (o *OSDCollector) collectOSDTreeDown(ch chan<- prometheus.Metric) error {
	cmd := o.cephOSDTreeCommand("down")
	buff, _, err := o.conn.MonCommand(cmd)
	if err != nil {
		o.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	osdDown := &cephOSDTreeDown{}
	if err := json.Unmarshal(buff, osdDown); err != nil {
		return err
	}

	downItems := append(osdDown.Nodes, osdDown.Stray...)
	for _, downItem := range downItems {
		if downItem.Type != "osd" {
			continue
		}

		osdName := downItem.Name
		lb := o.getOSDLabelFromName(osdName)

		ch <- prometheus.MustNewConstMetric(o.OSDDownDesc, prometheus.GaugeValue, 1,
			downItem.Status,
			osdName,
			lb.DeviceClass,
			lb.Host,
			lb.Root,
			lb.Rack)
	}

	return nil
}

func (o *OSDCollector) collectOSDDump() error {
	cmd := o.cephOSDDump()
	buff, _, err := o.conn.MonCommand(cmd)
	if err != nil {
		o.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	osdDump := cephOSDDump{}
	if err := json.Unmarshal(buff, &osdDump); err != nil {
		return err
	}

	osdFullRatio, err := osdDump.FullRatio.Float64()
	if err != nil {
		return err
	}
	osdNearFullRatio, err := osdDump.NearFullRatio.Float64()
	if err != nil {
		return err
	}
	osdBackfillFullRatio, err := osdDump.BackfillFullRatio.Float64()
	if err != nil {
		return err
	}
	o.OSDFullRatio.Set(osdFullRatio)
	o.OSDNearFullRatio.Set(osdNearFullRatio)
	o.OSDBackfillFullRatio.Set(osdBackfillFullRatio)
	o.PgUpmapItemsTotal.Set(float64(len(osdDump.PgUpmapItems)))

	for _, dumpInfo := range osdDump.OSDs {
		osdID, err := dumpInfo.OSD.Int64()
		if err != nil {
			return err
		}
		osdName := fmt.Sprintf(osdLabelFormat, osdID)
		lb := o.getOSDLabelFromID(osdID)

		in, err := dumpInfo.In.Float64()
		if err != nil {
			return err
		}

		o.OSDIn.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(in)

		up, err := dumpInfo.Up.Float64()
		if err != nil {
			return err
		}

		o.OSDUp.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(up)

		o.OSDFull.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(0)
		o.OSDNearFull.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(0)
		o.OSDBackfillFull.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(0)
		for _, state := range dumpInfo.State {
			switch state {
			case "full":
				o.OSDFull.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(1)
			case "nearfull":
				o.OSDNearFull.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(1)
			case "backfillfull":
				o.OSDBackfillFull.WithLabelValues(osdName, lb.DeviceClass, lb.Host, lb.Rack, lb.Root).Set(1)
			}
		}
	}

	return nil

}

func (o *OSDCollector) performPGDumpBrief() error {
	args := o.cephPGDumpCommand()
	buf, _, err := o.conn.MgrCommand(args)
	if err != nil {
		o.logger.WithError(err).WithField(
			"args", string(bytes.Join(args, []byte(","))),
		).Error("error executing mgr command")

		return err
	}

	o.pgDumpBrief = cephPGDumpBrief{}
	if err := json.Unmarshal(buf, &o.pgDumpBrief); err != nil {
		return err
	}

	return nil
}

func (o *OSDCollector) collectOSDScrubState(ch chan<- prometheus.Metric) error {
	// need to reset the PG scrub state since the scrub might have ended within
	// the last prom scrape interval.
	// This forces us to report scrub state on all previously discovered OSDs We
	// may be able to remove the "cache" when using Prometheus 2.0 if we can
	// tune how unreported/abandoned gauges are treated (ie set to 0).
	for i := range o.osdScrubCache {
		o.osdScrubCache[i] = scrubStateIdle
	}

	for _, pg := range o.pgDumpBrief.PGStats {
		if strings.Contains(pg.State, "scrubbing") {
			scrubState := scrubStateScrubbing
			if strings.Contains(pg.State, "deep") {
				scrubState = scrubStateDeepScrubbing
			}

			for _, osd := range pg.Acting {
				o.osdScrubCache[osd] = scrubState
			}
		}
	}

	for i, v := range o.osdScrubCache {
		lb := o.getOSDLabelFromID(int64(i))
		ch <- prometheus.MustNewConstMetric(
			o.ScrubbingStateDesc,
			prometheus.GaugeValue,
			float64(v),
			fmt.Sprintf(osdLabelFormat, i),
			lb.DeviceClass,
			lb.Host,
			lb.Root,
			lb.Root)
	}

	return nil
}

func (o *OSDCollector) cephOSDDump() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd dump",
		"format": jsonFormat,
	})
	if err != nil {
		o.logger.WithError(err).Panic("error marshalling ceph osd dump")
	}
	return cmd
}

func (o *OSDCollector) cephOSDDFCommand() [][]byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd df",
		"format": jsonFormat,
	})
	if err != nil {
		o.logger.WithError(err).Panic("error marshalling ceph osd df")
	}
	return [][]byte{cmd}
}

func (o *OSDCollector) cephOSDPerfCommand() [][]byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd perf",
		"format": jsonFormat,
	})
	if err != nil {
		o.logger.WithError(err).Panic("error marshalling ceph osd perf")
	}
	return [][]byte{cmd}
}

func (o *OSDCollector) cephOSDTreeCommand(states ...string) []byte {
	req := map[string]interface{}{
		"prefix": "osd tree",
		"format": jsonFormat,
	}
	if len(states) > 0 {
		req["states"] = states
	}

	cmd, err := json.Marshal(req)
	if err != nil {
		o.logger.WithError(err).Panic("error marshalling ceph osd tree")
	}
	return cmd
}

func (o *OSDCollector) cephPGDumpCommand() [][]byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix":       "pg dump",
		"dumpcontents": []string{"pgs_brief"},
		"format":       jsonFormat,
	})
	if err != nil {
		o.logger.WithError(err).Panic("error marshalling ceph pg dump")
	}
	return [][]byte{cmd}
}

func (o *OSDCollector) collectPGStates(ch chan<- prometheus.Metric) error {
	// - See if there are PGs that we're tracking that are now active
	// - See if there are new ones to add
	// - Find the oldest one
	now := time.Now()
	oldestTime := now

	for _, pg := range o.pgDumpBrief.PGStats {
		// If we were tracking it, and it's now active, remove it
		active := strings.Contains(pg.State, "active")
		if active {
			delete(o.oldestInactivePGMap, pg.PGID)
			continue
		}

		// Now see if it's not here, we'll need to track it now
		pgTime, ok := o.oldestInactivePGMap[pg.PGID]
		if !ok {
			pgTime = now
			o.oldestInactivePGMap[pg.PGID] = now
		}

		// And finally, track our oldest time
		if pgTime.Before(oldestTime) {
			oldestTime = pgTime
		}
	}

	o.OldestInactivePG.Set(float64(now.Unix() - oldestTime.Unix()))
	return nil
}

// Describe sends the descriptors of each OSDCollector related metrics we have
// defined to the provided Prometheus channel.
func (o *OSDCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range o.collectorList() {
		metric.Describe(ch)
	}
	ch <- o.OSDDownDesc
	ch <- o.ScrubbingStateDesc
	ch <- o.PGObjectsRecoveredDesc
}

// Collect sends all the collected metrics to the provided Prometheus channel.
// It requires the caller to handle synchronization.
func (o *OSDCollector) Collect(ch chan<- prometheus.Metric, version *Version) {
	// Reset daemon specifc metrics; daemons can leave the cluster
	o.CrushWeight.Reset()
	o.Depth.Reset()
	o.Reweight.Reset()
	o.Bytes.Reset()
	o.UsedBytes.Reset()
	o.AvailBytes.Reset()
	o.Utilization.Reset()
	o.Variance.Reset()
	o.Pgs.Reset()
	o.CommitLatency.Reset()
	o.ApplyLatency.Reset()
	o.OSDIn.Reset()
	o.OSDUp.Reset()
	o.buildOSDLabelCache()

	o.logger.Debug("collecting OSD perf metrics")
	if err := o.collectOSDPerf(); err != nil {
		o.logger.WithError(err).Error("error collecting OSD perf metrics")
	}

	o.logger.Debug("collecting OSD dump metrics")
	if err := o.collectOSDDump(); err != nil {
		o.logger.WithError(err).Error("error collecting OSD dump metrics")
	}

	o.logger.Debug("collecting OSD df metrics")
	if err := o.collectOSDDF(); err != nil {
		o.logger.WithError(err).Error("error collecting OSD df metrics")
	}

	o.logger.Debug("collecting OSD tree down metrics")
	if err := o.collectOSDTreeDown(ch); err != nil {
		o.logger.WithError(err).Error("error collecting OSD tree down metrics")
	}

	o.logger.Debug("collecting PG dump metrics")
	if err := o.performPGDumpBrief(); err != nil {
		o.logger.WithError(err).Error("error collecting PG dump metrics")
	}

	o.logger.Debug("collecting OSD scrub metrics")
	if err := o.collectOSDScrubState(ch); err != nil {
		o.logger.WithError(err).Error("error collecting OSD scrub metrics")
	}

	o.logger.Debug("collecting PG states")
	if err := o.collectPGStates(ch); err != nil {
		o.logger.WithError(err).Error("error collecting PG state metrics")
	}

	for _, metric := range o.collectorList() {
		metric.Collect(ch)
	}
}
