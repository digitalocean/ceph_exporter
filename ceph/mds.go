package ceph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	cephCmd                      = "/usr/bin/ceph"
	mdsBackgroundCollectInterval = 5 * time.Minute
)

const (
	MDSModeDisabled   = 0
	MDSModeForeground = 1
	MDSModeBackground = 2
)

type mdsStat struct {
	FSMap struct {
		Filesystems []struct {
			MDSMap struct {
				FSName string `json:"fs_name"`
				Info   map[string]struct {
					GID   uint   `json:"gid"`
					Name  string `json:"name"`
					Rank  int    `json:"rank"`
					State string `json:"state"`
				} `json:"info"`
			} `json:"mdsmap"`
		} `json:"filesystems"`
	} `json:"fsmap"`
}

// runMDSStat will run mds stat and get all info from the MDSs within the ceph cluster.
func runMDSStat(ctx context.Context, config, user string) ([]byte, error) {
	return exec.CommandContext(ctx, cephCmd, "-c", config, "-n", fmt.Sprintf("client.%s", user), "mds", "stat", "--format", "json").Output()
}

// runCephHealthDetail will run health detail and get info specific to MDSs within the ceph cluster.
func runCephHealthDetail(ctx context.Context, config, user string) ([]byte, error) {
	return exec.CommandContext(ctx, cephCmd, "-c", config, "-n", fmt.Sprintf("client.%s", user), "health", "detail", "--format", "json").Output()
}

// runMDSStatus will run status command on the MDS to get it's info.
func runMDSStatus(ctx context.Context, config, user, mds string) ([]byte, error) {
	return exec.CommandContext(ctx, cephCmd, "-c", config, "-n", "tell", mds, "status").Output()
}

// runBlockedOpsCheck will run blocked ops on MDSs and get any ops that are blocked for that MDS.
func runBlockedOpsCheck(ctx context.Context, config, user, mds string) ([]byte, error) {
	return exec.CommandContext(ctx, cephCmd, "-c", config, "-n", "tell", mds, "dump_blocked_ops").Output()
}

// MDSCollector collects metrics from the MDS daemons.
type MDSCollector struct {
	config     string
	user       string
	background bool
	logger     *logrus.Logger
	ch         chan prometheus.Metric

	// MDSState reports the state of MDS process running.
	MDSState *prometheus.Desc

	// MDSBlockedOPs reports the slow or blocked ops on an MDS.
	MDSBlockedOps *prometheus.Desc

	runMDSStatFn          func(context.Context, string, string) ([]byte, error)
	runCephHealthDetailFn func(context.Context, string, string) ([]byte, error)
	runMDSStatusFn        func(context.Context, string, string, string) ([]byte, error)
	runBlockedOpsCheckFn  func(context.Context, string, string, string) ([]byte, error)
}

// NewMDSCollector creates an instance of the MDSCollector and instantiates
// the individual metrics that we can collect from the MDS daemons.
func NewMDSCollector(exporter *Exporter, background bool) *MDSCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	mds := &MDSCollector{
		config:                exporter.Config,
		user:                  exporter.User,
		background:            background,
		logger:                exporter.Logger,
		ch:                    make(chan prometheus.Metric, 100),
		runMDSStatFn:          runMDSStat,
		runCephHealthDetailFn: runCephHealthDetail,
		runMDSStatusFn:        runMDSStatus,
		runBlockedOpsCheckFn:  runBlockedOpsCheck,

		MDSState: prometheus.NewDesc(
			fmt.Sprintf("%s_%s", cephNamespace, "mds_daemon_state"),
			"MDS Daemon State",
			[]string{"fs", "name", "rank", "state"},
			labels,
		),
		MDSBlockedOps: prometheus.NewDesc(
			fmt.Sprintf("%s_%s", cephNamespace, "mds_blocked_ops"),
			"MDS Blocked Ops",
			[]string{"fs", "name", "state", "optype", "fs_optype", "flag_point"},
			labels,
		),
	}

	return mds
}

func (m *MDSCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{}
}

func (m *MDSCollector) descriptorList() []*prometheus.Desc {
	return []*prometheus.Desc{
		m.MDSState,
	}
}

func (m *MDSCollector) backgroundCollect() {
	defer close(m.ch)
	for {
		m.logger.WithField("background", m.background).Debug("collecting MDS stats")
		err := m.collect()
		if err != nil {
			m.logger.WithField("background", m.background).WithError(err).Error("error collecting MDS stats")
		}
		time.Sleep(mdsBackgroundCollectInterval)
	}
}

func (m *MDSCollector) collect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	data, err := m.runMDSStatFn(ctx, m.config, m.user)
	if err != nil {
		return fmt.Errorf("failed getting mds stat: %w", err)
	}

	ms := &mdsStat{}

	err = json.Unmarshal(data, ms)
	if err != nil {
		return fmt.Errorf("failed unmarshalling mds stat json: %w", err)
	}

	for _, fs := range ms.FSMap.Filesystems {
		for _, info := range fs.MDSMap.Info {
			select {
			case m.ch <- prometheus.MustNewConstMetric(
				m.MDSState,
				prometheus.GaugeValue,
				float64(1),
				fs.MDSMap.FSName,
				info.Name,
				strconv.Itoa(info.Rank),
				info.State,
			):
			default:
			}
		}
	}

	m.collectMDSSlowOps()

	return nil
}

// Describe sends the descriptors of each MDSCollector related metrics we have defined
// to the provided prometheus channel.
func (m *MDSCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range m.collectorList() {
		metric.Describe(ch)
	}

	for _, metric := range m.descriptorList() {
		ch <- metric
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (m *MDSCollector) Collect(ch chan<- prometheus.Metric, version *Version) {
	if !m.background {
		m.logger.WithField("background", m.background).Debug("collecting MDS stats")
		err := m.collect()
		if err != nil {
			m.logger.WithField("background", m.background).WithError(err).Error("error collecting MDS stats")
		}
	}

	if m.background {
		go m.backgroundCollect()
	}

	for _, metric := range m.collectorList() {
		metric.Collect(ch)
	}

	for {
		select {
		case cc, ok := <-m.ch:
			if ok {
				ch <- cc
			}
		default:
			return
		}
	}
}

type healthDetailCheck struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Severity string `json:"severity"`
		Summary  struct {
			Message string `json:"message"`
			Count   int    `json:"count"`
		} `json:"summary"`
		Detail []struct {
			Message string `json:"message"`
		} `json:"detail"`
		Muted bool `json:"muted"`
	} `json:"checks"`
}

type mdsStatus struct {
	ClusterFsid        string  `json:"cluster_fsid"`
	Whoami             int     `json:"whoami"`
	ID                 int64   `json:"id"`
	WantState          string  `json:"want_state"`
	State              string  `json:"state"`
	FsName             string  `json:"fs_name"`
	RankUptime         float64 `json:"rank_uptime"`
	MdsmapEpoch        int     `json:"mdsmap_epoch"`
	OsdmapEpoch        int     `json:"osdmap_epoch"`
	OsdmapEpochBarrier int     `json:"osdmap_epoch_barrier"`
	Uptime             float64 `json:"uptime"`
}

type mdsSlowOp struct {
	Ops []struct {
		// Custom fields for easy parsing by caller.
		MDSName, CephFSOpType string

		// CephFS fields.
		Description string  `json:"description"`
		InitiatedAt string  `json:"initiated_at"`
		Age         float64 `json:"age"`
		Duration    float64 `json:"duration"`
		TypeData    struct {
			FlagPoint  string `json:"flag_point"`
			Reqid      string `json:"reqid"`
			OpType     string `json:"op_type"`
			ClientInfo struct {
				Client string `json:"client"`
				Tid    int    `json:"tid"`
			} `json:"client_info"`
			Events []struct {
				Time  string `json:"time"`
				Event string `json:"event"`
			} `json:"events"`
		} `json:"type_data,omitempty"`
	} `json:"ops"`
	ComplaintTime int `json:"complaint_time"`
	NumBlockedOps int `json:"num_blocked_ops"`
}

func (m *MDSCollector) collectMDSSlowOps() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	data, err := runCephHealthDetail(ctx, m.config, m.user)
	if err != nil {
		m.logger.WithError(err).Error("failed getting health detail")
		return
	}

	hc := &healthDetailCheck{}

	err = json.Unmarshal(data, hc)
	if err != nil {
		m.logger.WithError(err).Error("failed unmarshalling health detail")
		return
	}

	check, ok := hc.Checks["MDS_SLOW_REQUEST"]
	if !ok {
		// No slow requests! Yay!
		return
	}

	for _, cc := range check.Detail {
		mdsNameParts := strings.Split(cc.Message, "(")
		if len(mdsNameParts) != 2 {
			m.logger.WithError(
				errors.New("incorrect part count"),
			).WithFields(logrus.Fields{
				"message": cc.Message,
			}).Error("invalid mds slow request message found, check syntax")
			continue
		}

		mdsName := mdsNameParts[0]

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		data, err := runMDSStatus(ctx, m.config, m.user, mdsName)
		if err != nil {
			m.logger.WithField("mds", mdsName).WithError(err).Error("failed getting status from mds")
			return
		}

		mss := &mdsStatus{}

		err = json.Unmarshal(data, mss)
		if err != nil {
			m.logger.WithField("mds", mdsName).WithError(err).Error("failed unmarshalling mds status")
			return
		}

		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		data, err = runBlockedOpsCheck(ctx, m.config, m.user, mdsName)
		if err != nil {
			m.logger.WithField("mds", mdsName).WithError(err).Error("failed getting blocked ops from mds")
			return
		}

		mso := &mdsSlowOp{}

		err = json.Unmarshal(data, mso)
		if err != nil {
			m.logger.WithField("mds", mdsName).WithError(err).Error("failed unmarshalling mds blocked ops")
			return
		}

		for _, op := range mso.Ops {
			if op.TypeData.OpType == "client_request" {
				opd, err := extractOpFromDescription(op.Description)
				if err != nil {
					m.logger.WithField("mds", mdsName).WithError(err).Error("failed parsing blocked ops description")
					continue
				}

				select {
				case m.ch <- prometheus.MustNewConstMetric(
					m.MDSBlockedOps,
					prometheus.CounterValue,
					1,
					mss.FsName,
					mdsName,
					mss.State,
					op.TypeData.OpType,
					opd.fsOpType,
					op.TypeData.FlagPoint,
				):
				default:
				}

				continue
			}

			select {
			case m.ch <- prometheus.MustNewConstMetric(
				m.MDSBlockedOps,
				prometheus.CounterValue,
				1,
				mss.FsName,
				mdsName,
				mss.State,
				op.TypeData.OpType,
				"",
				op.TypeData.FlagPoint,
			):
			default:
			}
		}

	}
}

type opDesc struct {
	fsOpType string
	inode    string
	clientID string
}

// extractOpFromDescription is designed to extract the fs optype from a given slow/blocked
// ops description.
//
// For e.g. given the description as follows:
//
//	"client_request(client.20001974182:344151 rmdir #0x10000000030/72a26231-ac24-4f69-9350-8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})"
//
// we should be able to extract the following fs optype out of it:
//
//	"rmdir"
func extractOpFromDescription(desc string) (*opDesc, error) {
	parts := strings.Fields(desc)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid fs description: %q", desc)
	}

	fsoptype, inode := parts[1], parts[2]
	inode = strings.TrimLeft(inode, "#0x")
	inode = strings.Split(inode, "/")[0]

	_, err := strconv.ParseUint(inode, 16, 14)
	if err != nil {
		return nil, fmt.Errorf("invalid inode, expected hex instead got %q: %w", inode, err)
	}

	clientIDParts := strings.Split(parts[0], "(")
	if len(clientIDParts) != 2 {
		return nil, fmt.Errorf("invalid client request format: %q", parts[0])
	}

	clientIDParts = strings.Split(clientIDParts[1], ":")
	if len(clientIDParts) != 2 {
		return nil, fmt.Errorf("invalid client id format: %q", clientIDParts[1])
	}

	clientIDParts = strings.Split(clientIDParts[0], ".")
	if len(clientIDParts) != 2 {
		return nil, fmt.Errorf("invalid client id string: %q", clientIDParts[0])
	}

	return &opDesc{
		fsOpType: fsoptype,
		inode:    inode,
		clientID: clientIDParts[1],
	}, nil
}
