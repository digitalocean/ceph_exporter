package collectors

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

type OsdCollector struct {
	conn Conn

	CrushWeight *prometheus.GaugeVec

	Depth *prometheus.GaugeVec

	Reweight *prometheus.GaugeVec

	KB *prometheus.GaugeVec

	UsedKB *prometheus.GaugeVec

	AvailKB *prometheus.GaugeVec

	Utilization *prometheus.GaugeVec

	Pgs *prometheus.GaugeVec

	CommitLatency *prometheus.GaugeVec

	ApplyLatency *prometheus.GaugeVec

	OsdIn *prometheus.GaugeVec

	OsdUp *prometheus.GaugeVec

	TotalKB prometheus.Gauge

	TotalUsedKB prometheus.Gauge

	TotalAvailKB prometheus.Gauge

	AverageUtil prometheus.Gauge
}

func NewOsdCollector(conn Conn) *OsdCollector {
	return &OsdCollector{
		conn: conn,

		CrushWeight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_crush_weight",
				Help:      "OSD Crush Weight",
			},
			[]string{"osd"},
		),

		Depth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_depth",
				Help:      "OSD Depth",
			},
			[]string{"osd"},
		),

		Reweight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_reweight",
				Help:      "OSD Reweight",
			},
			[]string{"osd"},
		),

		KB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_kb",
				Help:      "OSD Total KB",
			},
			[]string{"osd"},
		),

		UsedKB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_kb_used",
				Help:      "OSD Used Storage in KB",
			},
			[]string{"osd"},
		),

		AvailKB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_kb_avail",
				Help:      "OSD Available Storage in KB",
			},
			[]string{"osd"},
		),

		Utilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_utilization",
				Help:      "OSD Utilization",
			},
			[]string{"osd"},
		),

		Pgs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_pgs",
				Help:      "OSD Placement Group Count",
			},
			[]string{"osd"},
		),

		TotalKB: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_total_kb",
				Help:      "OSD Total Storage KB",
			},
		),
		TotalUsedKB: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_total_kb_used",
				Help:      "OSD Total Used Storage KB",
			},
		),

		TotalAvailKB: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_total_kb_avail",
				Help:      "OSD Total Available Storage KB ",
			},
		),

		AverageUtil: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_average_utilization",
				Help:      "OSD Average Utilization",
			},
		),

		CommitLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_perf_commit_latency_ms",
				Help:      "OSD Perf Commit Latency",
			},
			[]string{"osd"},
		),

		ApplyLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_perf_apply_latency_ms",
				Help:      "OSD Perf Apply Latency",
			},
			[]string{"osd"},
		),

		OsdIn: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_in",
				Help:      "OSD In Status",
			},
			[]string{"osd"},
		),

		OsdUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cephNamespace,
				Name:      "osd_up",
				Help:      "OSD Up Status",
			},
			[]string{"osd"},
		),
	}
}

func (o *OsdCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		o.CrushWeight,
		o.Depth,
		o.Reweight,
		o.KB,
		o.UsedKB,
		o.AvailKB,
		o.Utilization,
		o.Pgs,
		o.TotalKB,
		o.TotalUsedKB,
		o.TotalAvailKB,
		o.AverageUtil,
		o.CommitLatency,
		o.ApplyLatency,
		o.OsdIn,
		o.OsdUp,
	}
}

type cephOsdDf struct {
	OsdNodes []struct {
		Name        string      `json:"name"`
		CrushWeight json.Number `json:"crush_weight"`
		Depth       json.Number `json:"depth"`
		Reweight    json.Number `json:"reweight"`
		KB          json.Number `json:"kb"`
		UsedKB      json.Number `json:"kb_used"`
		AvailKB     json.Number `json:"kb_avail"`
		Utilization json.Number `json:"utilization"`
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
		Id    json.Number `json:"id"`
		Stats struct {
			CommitLatency json.Number `json:"commit_latency_ms"`
			ApplyLatency  json.Number `json:"apply_latency_ms"`
		} `json:"perf_stats"`
	} `json:"osd_perf_infos"`
}

type cephOsdDump struct {
	Osds []struct {
		Osd json.Number `json:"osd"`
		Up  json.Number `json:"up"`
		In  json.Number `json:"in"`
	} `json:"osds"`
}

func (o *OsdCollector) collect() error {
	cmd := o.cephOSDDfCommand()
	buf, _, err := o.conn.MonCommand(cmd)

	osdDf := &cephOsdDf{}
	if err := json.Unmarshal(buf, osdDf); err != nil {
		return err
	}

	for _, node := range osdDf.OsdNodes {

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

		kb, err := node.KB.Float64()
		if err != nil {
			return nil
		}

		o.KB.WithLabelValues(node.Name).Set(kb)

		usedKb, err := node.UsedKB.Float64()
		if err != nil {
			return err
		}

		o.UsedKB.WithLabelValues(node.Name).Set(usedKb)

		availKb, err := node.AvailKB.Float64()
		if err != nil {
			return err
		}

		o.AvailKB.WithLabelValues(node.Name).Set(availKb)

		util, err := node.Utilization.Float64()
		if err != nil {
			return err
		}

		o.Utilization.WithLabelValues(node.Name).Set(util)

		pgs, err := node.Pgs.Float64()
		if err != nil {
			return err
		}

		o.Pgs.WithLabelValues(node.Name).Set(pgs)

	}

	totalKb, err := osdDf.Summary.TotalKB.Float64()
	if err != nil {
		return nil
	}

	o.TotalKB.Set(totalKb)

	totalUsedKb, err := osdDf.Summary.TotalUsedKB.Float64()
	if err != nil {
		return err
	}

	o.TotalUsedKB.Set(totalUsedKb)

	totalAvailKb, err := osdDf.Summary.TotalAvailKB.Float64()
	if err != nil {
		return err
	}

	o.TotalAvailKB.Set(totalAvailKb)

	averageUtil, err := osdDf.Summary.AverageUtil.Float64()
	if err != nil {
		return nil
	}

	o.AverageUtil.Set(averageUtil)

	return nil

}

func (o *OsdCollector) collectOsdPerf() error {
	osdPerfCmd := o.cephOSDPerfCommand()
	buf, _, _ := o.conn.MonCommand(osdPerfCmd)

	osdPerf := &cephPerfStat{}
	if err := json.Unmarshal(buf, osdPerf); err != nil {
		return err
	}

	for _, perfStat := range osdPerf.PerfInfo {
		osdId, err := perfStat.Id.Int64()
		if err != nil {
			return err
		}
		osdName := fmt.Sprintf("osd.%v", osdId)

		commitLatency, err := perfStat.Stats.CommitLatency.Float64()
		if err != nil {
			return err
		}
		o.CommitLatency.WithLabelValues(osdName).Set(commitLatency)

		applyLatency, err := perfStat.Stats.ApplyLatency.Float64()
		if err != nil {
			return err
		}
		o.ApplyLatency.WithLabelValues(osdName).Set(applyLatency)
	}

	return nil
}

func (o *OsdCollector) collectOsdDump() error {
	osdDumpCmd := o.cephOsdDump()
	buff, _, _ := o.conn.MonCommand(osdDumpCmd)

	osdDump := &cephOsdDump{}
	if err := json.Unmarshal(buff, osdDump); err != nil {
		return err
	}

	for _, dumpInfo := range osdDump.Osds {
		osdId, err := dumpInfo.Osd.Int64()
		if err != nil {
			return err
		}
		osdName := fmt.Sprintf("osd.%v", osdId)

		in, err := dumpInfo.In.Float64()
		if err != nil {
			return err
		}

		o.OsdIn.WithLabelValues(osdName).Set(in)

		up, err := dumpInfo.Up.Float64()
		if err != nil {
			return err
		}

		o.OsdUp.WithLabelValues(osdName).Set(up)
	}

	return nil

}

func (o *OsdCollector) cephOsdDump() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd dump",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

func (o *OsdCollector) cephOSDDfCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd df",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

func (o *OsdCollector) cephOSDPerfCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd perf",
		"format": "json",
	})
	if err != nil {
		panic(err)
	}
	return cmd
}

func (o *OsdCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range o.collectorList() {
		metric.Describe(ch)
	}

}

func (o *OsdCollector) Collect(ch chan<- prometheus.Metric) {
	if err := o.collect(); err != nil {
		log.Println("failed collecting osd metrics:", err)
		return
	}

	if err := o.collectOsdPerf(); err != nil {
		log.Println("failed collecting cluster osd perf stats:", err)
	}

	if err := o.collectOsdDump(); err != nil {
		log.Println("failed collecting cluster osd dump", err)
	}

	for _, metric := range o.collectorList() {
		metric.Collect(ch)
	}

}
