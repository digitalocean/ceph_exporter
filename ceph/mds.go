package ceph

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
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
func runMDSStat(config, user string) ([]byte, error) {
	var (
		out []byte
		err error
	)

	if out, err = exec.Command(cephCmd, "-c", config, "-n", fmt.Sprintf("client.%s", user), "mds", "stat", "--format", "json").Output(); err != nil {
		return nil, err
	}

	return out, nil
}

// MDSCollector collects metrics from the MDS daemons.
type MDSCollector struct {
	config     string
	user       string
	background bool
	logger     *logrus.Logger

	// MDSState reports the state of MDS process running.
	MDSState *prometheus.Desc

	runMDSStatFn func(string, string) ([]byte, error)
}

// NewMDSCollector creates an instance of the MDSCollector and instantiates
// the individual metrics that we can collect from the MDS daemons.
func NewMDSCollector(exporter *Exporter, background bool) *MDSCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	mds := &MDSCollector{
		config:       exporter.Config,
		user:         exporter.User,
		background:   background,
		logger:       exporter.Logger,
		runMDSStatFn: runMDSStat,

		MDSState: prometheus.NewDesc(
			fmt.Sprintf("%s_%s", cephNamespace, "mds_daemon_state"),
			"MDS Daemon State",
			[]string{"fs", "name", "rank", "state"},
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

func (m *MDSCollector) backgroundCollect(ch chan<- prometheus.Metric) error {
	for {
		m.logger.WithField("background", m.background).Debug("collecting MDS stats")
		err := m.collect(ch)
		if err != nil {
			m.logger.WithField("background", m.background).WithError(err).Error("error collecting MDS stats")
		}
		time.Sleep(mdsBackgroundCollectInterval)
	}
}

func (m *MDSCollector) collect(ch chan<- prometheus.Metric) error {
	data, err := m.runMDSStatFn(m.config, m.user)
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
			ch <- prometheus.MustNewConstMetric(
				m.MDSState,
				prometheus.GaugeValue,
				float64(1),
				fs.MDSMap.FSName,
				info.Name,
				strconv.Itoa(info.Rank),
				info.State,
			)
		}
	}

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
		err := m.collect(ch)
		if err != nil {
			m.logger.WithField("background", m.background).WithError(err).Error("error collecting MDS stats")
		}
	}

	if m.background {
		go m.backgroundCollect(ch)
	}

	for _, metric := range m.collectorList() {
		metric.Collect(ch)
	}
}
