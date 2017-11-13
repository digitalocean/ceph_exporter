package collectors

import (
	"encoding/json"
	"fmt"
	"log"
	"bytes"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
)

// OSDCollector displays statistics about OSD in the ceph cluster.
// An important aspect of monitoring OSDs is to ensure that when the cluster is up and
// running that all OSDs that are in the cluster are up and running, too
type OSDCollector struct {
	conn Conn

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

	// Pgs displays total no. of placement groups in the OSD.
	// Available in Ceph Jewel version.
	Pgs *prometheus.GaugeVec

	// CommitLatency displays in seconds how long it takes for an operation to be applied to disk
	CommitLatency *prometheus.GaugeVec

	// ApplyLatency displays in seconds how long it takes to get applied to the backing filesystem
	ApplyLatency *prometheus.GaugeVec

	// OSDIn displays the In state of the OSD
	OSDIn *prometheus.GaugeVec

	// OSDUp displays the Up state of the OSD
	OSDUp *prometheus.GaugeVec

	// OSDTree used to display the location of the OSD in the CRUSH tree
	OSDTree *prometheus.GaugeVec

	// TotalBytes displays total bytes in all OSDs
	TotalBytes prometheus.Gauge

	// TotalUsedBytes displays total used bytes in all OSDs
	TotalUsedBytes prometheus.Gauge

	// TotalAvailBytes displays total available bytes in all OSDs
	TotalAvailBytes prometheus.Gauge

	// AverageUtil displays average utilization in all OSDs
	AverageUtil prometheus.Gauge
}

//NewOSDCollector creates an instance of the OSDCollector and instantiates
// the individual metrics that show information about the OSD.
func NewOSDCollector(conn Conn, cluster string) *OSDCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = cluster

	return &OSDCollector{
		conn: conn,

		CrushWeight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_crush_weight",
				Help:        "OSD Crush Weight",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		Depth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_depth",
				Help:        "OSD Depth",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		Reweight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_reweight",
				Help:        "OSD Reweight",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		Bytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_bytes",
				Help:        "OSD Total Bytes",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		UsedBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_used_bytes",
				Help:        "OSD Used Storage in Bytes",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		AvailBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_avail_bytes",
				Help:        "OSD Available Storage in Bytes",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		Utilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_utilization",
				Help:        "OSD Utilization",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		Variance: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_variance",
				Help:        "OSD Variance",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		Pgs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_pgs",
				Help:        "OSD Placement Group Count",
				ConstLabels: labels,
			},
			[]string{"osd"},
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
			[]string{"osd"},
		),

		ApplyLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_perf_apply_latency_seconds",
				Help:        "OSD Perf Apply Latency",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		OSDIn: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_in",
				Help:        "OSD In Status",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		OSDUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_up",
				Help:        "OSD Up Status",
				ConstLabels: labels,
			},
			[]string{"osd"},
		),

		OSDTree: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osd_tree",
				Help:        "topology label points to where the osd exists in tree",
				ConstLabels: labels,
			},
			[]string{
				"osd",
				"topology",
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
		o.TotalBytes,
		o.TotalUsedBytes,
		o.TotalAvailBytes,
		o.AverageUtil,
		o.CommitLatency,
		o.ApplyLatency,
		o.OSDIn,
		o.OSDUp,
		o.OSDTree,
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

type cephOSDDump struct {
	OSDs []struct {
		OSD json.Number `json:"osd"`
		Up  json.Number `json:"up"`
		In  json.Number `json:"in"`
	} `json:"osds"`
}

type cephOSDTreeNode struct {
	ID       json.Number `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	TypeID   json.Number `json:"type_id"`
	Children []int64     `json:"children"`
	Exists   json.Number `json:"exists"`
	Status   string      `json:"status"`
}

type cephOSDTree struct {
	OSDNodes []cephOSDTreeNode `json:"nodes"`
}

func (o *OSDCollector) collect() error {
	cmd := o.cephOSDDFCommand()

	buf, _, err := o.conn.MonCommand(cmd)
	if err != nil {
		log.Println("[ERROR] Unable to collect data from ceph osd df", err)
		return err
	}

	// Workaround for Ceph Jewel after 10.2.5 produces invalid json when osd is out
	buf = bytes.Replace(buf, []byte("-nan"), []byte("0"), -1)

	osdDF := &cephOSDDF{}
	if err := json.Unmarshal(buf, osdDF); err != nil {
		return err
	}

	for _, node := range osdDF.OSDNodes {

		crushWeight, err := node.CrushWeight.Float64()
		if err != nil {
			return err
		}

		o.CrushWeight.WithLabelValues(node.Name).Set(crushWeight)

		depth, err := node.Depth.Float64()
		if err != nil {

			return err
		}

		o.Depth.WithLabelValues(node.Name).Set(depth)

		reweight, err := node.Reweight.Float64()
		if err != nil {
			return err
		}

		o.Reweight.WithLabelValues(node.Name).Set(reweight)

		osdKB, err := node.KB.Float64()
		if err != nil {
			return nil
		}

		o.Bytes.WithLabelValues(node.Name).Set(osdKB * 1e3)

		usedKB, err := node.UsedKB.Float64()
		if err != nil {
			return err
		}

		o.UsedBytes.WithLabelValues(node.Name).Set(usedKB * 1e3)

		availKB, err := node.AvailKB.Float64()
		if err != nil {
			return err
		}

		o.AvailBytes.WithLabelValues(node.Name).Set(availKB * 1e3)

		util, err := node.Utilization.Float64()
		if err != nil {
			return err
		}

		o.Utilization.WithLabelValues(node.Name).Set(util)

		variance, err := node.Variance.Float64()
		if err != nil {
			return err
		}

		o.Variance.WithLabelValues(node.Name).Set(variance)

		pgs, err := node.Pgs.Float64()
		if err != nil {
			continue
		}

		o.Pgs.WithLabelValues(node.Name).Set(pgs)

	}

	totalKB, err := osdDF.Summary.TotalKB.Float64()
	if err != nil {
		return err
	}

	o.TotalBytes.Set(totalKB * 1e3)

	totalUsedKB, err := osdDF.Summary.TotalUsedKB.Float64()
	if err != nil {
		return err
	}

	o.TotalUsedBytes.Set(totalUsedKB * 1e3)

	totalAvailKB, err := osdDF.Summary.TotalAvailKB.Float64()
	if err != nil {
		return err
	}

	o.TotalAvailBytes.Set(totalAvailKB * 1e3)

	averageUtil, err := osdDF.Summary.AverageUtil.Float64()
	if err != nil {
		return err
	}

	o.AverageUtil.Set(averageUtil)

	return nil

}

func (o *OSDCollector) collectOSDPerf() error {
	osdPerfCmd := o.cephOSDPerfCommand()
	buf, _, err := o.conn.MonCommand(osdPerfCmd)
	if err != nil {
		log.Println("[ERROR] Unable to collect data from ceph osd perf", err)
		return err
	}

	osdPerf := &cephPerfStat{}
	if err := json.Unmarshal(buf, osdPerf); err != nil {
		return err
	}

	for _, perfStat := range osdPerf.PerfInfo {
		osdID, err := perfStat.ID.Int64()
		if err != nil {
			return err
		}
		osdName := fmt.Sprintf("osd.%v", osdID)

		commitLatency, err := perfStat.Stats.CommitLatency.Float64()
		if err != nil {
			return err
		}
		o.CommitLatency.WithLabelValues(osdName).Set(commitLatency / 1e3)

		applyLatency, err := perfStat.Stats.ApplyLatency.Float64()
		if err != nil {
			return err
		}
		o.ApplyLatency.WithLabelValues(osdName).Set(applyLatency / 1e3)
	}

	return nil
}

func (o *OSDCollector) collectOSDDump() error {
	osdDumpCmd := o.cephOSDDump()
	buff, _, err := o.conn.MonCommand(osdDumpCmd)
	if err != nil {
		log.Println("[ERROR] Unable to collect data from ceph osd dump", err)
		return err
	}

	osdDump := &cephOSDDump{}
	if err := json.Unmarshal(buff, osdDump); err != nil {
		return err
	}

	for _, dumpInfo := range osdDump.OSDs {
		osdID, err := dumpInfo.OSD.Int64()
		if err != nil {
			return err
		}
		osdName := fmt.Sprintf("osd.%v", osdID)

		in, err := dumpInfo.In.Float64()
		if err != nil {
			return err
		}

		o.OSDIn.WithLabelValues(osdName).Set(in)

		up, err := dumpInfo.Up.Float64()
		if err != nil {
			return err
		}

		o.OSDUp.WithLabelValues(osdName).Set(up)
	}

	return nil

}

func (o *OSDCollector) collectOSDTree() error {
	osdTreeCmd := o.cephOSDTreeCommand()
	buff, _, err := o.conn.MonCommand(osdTreeCmd)
	if err != nil {
		log.Println("[ERROR] Unable to collect data from ceph osd tree", err)
		return err
	}

	osdTree := &cephOSDTree{}
	if err := json.Unmarshal(buff, osdTree); err != nil {
		return err
	}

	// We need to build a lookup table first as the results returned by ceph
	// are not properly indexed.
	// We also need to keep track of all the children so we can identify the roots

	nodes := make(map[int64]int)
	is_child := make(map[int64]bool)
	for addr, osdTreeNode := range osdTree.OSDNodes {
		id, err := osdTreeNode.ID.Int64()
		if err != nil {
			return err
		}
		nodes[id] = addr
		if len(osdTreeNode.Children) > 0 {
			for _, child := range osdTreeNode.Children {
				is_child[child] = true
			}
		}
	}
	for id, addr := range nodes {
		if (!is_child[id]) {
			// We have a root so we need to walk it
			labels := make(prometheus.Labels)
			labels[osdTree.OSDNodes[addr].Type] = osdTree.OSDNodes[addr].Name
			o.recurseOSDTreeWalk(nodes, osdTree.OSDNodes, id, labels)
		}
	}

	return nil
}

func (o *OSDCollector) recurseOSDTreeWalk(nodes map[int64]int, osdNodes []cephOSDTreeNode, id int64, labels prometheus.Labels) error {
	addr := nodes[id]
	node := osdNodes[addr]
	if len(node.Children) > 0 {
		for _, child_id := range osdNodes[addr].Children {
			labels[node.Type] = node.Name
			o.recurseOSDTreeWalk(nodes, osdNodes, child_id, labels)
		}
	} else {
		val, err := node.Exists.Float64()
		if err != nil {
			return err
		}
		// Format the topology between commas so it is easy to parse and relabel by prometheus.
		// Golang maps have an undefined order so we will make it easy for prometheus and
		// sort the labels first before composing the topology label
		topology := ""
		var keys []string
		for label := range labels {
			keys = append(keys, label)
		}
		sort.Strings(keys)
		for _, key := range keys {
			topology = fmt.Sprintf("%s,%s=%s", topology, key, labels[key])
		}
		o.OSDTree.WithLabelValues(node.Name, fmt.Sprintf("%s,", topology)).Set(val)
	}
	return nil
}

func (o *OSDCollector) cephOSDDump() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd dump",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

func (o *OSDCollector) cephOSDDFCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd df",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

func (o *OSDCollector) cephOSDPerfCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd perf",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

func (o *OSDCollector) cephOSDTreeCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd tree",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

// Describe sends the descriptors of each OSDCollector related metrics we have defined
// to the provided prometheus channel.
func (o *OSDCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range o.collectorList() {
		metric.Describe(ch)
	}

}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (o *OSDCollector) Collect(ch chan<- prometheus.Metric) {

	if err := o.collectOSDTree(); err != nil {
		log.Println("failed collecting osd tree stats:", err)
	}

	if err := o.collectOSDPerf(); err != nil {
		log.Println("failed collecting osd perf stats:", err)
	}

	if err := o.collectOSDDump(); err != nil {
		log.Println("failed collecting osd dump:", err)
	}

	if err := o.collect(); err != nil {
		log.Println("failed collecting osd metrics:", err)
	}

	for _, metric := range o.collectorList() {
		metric.Collect(ch)
	}

}
