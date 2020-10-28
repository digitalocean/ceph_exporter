//   Copyright 2019 DigitalOcean
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

package collectors

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	poolReplicated = 1
	poolErasure    = 3
)

// PoolInfoCollector gives information about each pool that exists in a given
// ceph cluster.
type PoolInfoCollector struct {
	conn   Conn
	logger *logrus.Logger

	// PGNum contains the count of PGs allotted to a particular pool.
	PGNum *prometheus.GaugeVec

	// PlacementPGNum contains the count of PGs allotted to a particular pool
	// and used for placements.
	PlacementPGNum *prometheus.GaugeVec

	// MinSize shows minimum number of copies or chunks of an object
	// that need to be present for active I/O.
	MinSize *prometheus.GaugeVec

	// ActualSize shows total copies or chunks of an object that need to be
	// present for a healthy cluster.
	ActualSize *prometheus.GaugeVec

	// QuotaMaxBytes lists maximum amount of bytes of data allowed in a pool.
	QuotaMaxBytes *prometheus.GaugeVec

	// QuotaMaxObjects contains maximum amount of RADOS objects allowed in a pool.
	QuotaMaxObjects *prometheus.GaugeVec

	// StripeWidth contains width of a RADOS object in a pool.
	StripeWidth *prometheus.GaugeVec

	// ExpansionFactor Contains a float >= 1 that defines the EC or replication multiplier of a pool
	ExpansionFactor *prometheus.GaugeVec
}

// NewPoolInfoCollector displays information about each pool in the cluster.
func NewPoolInfoCollector(conn Conn, cluster string, logger *logrus.Logger) *PoolInfoCollector {
	var (
		subSystem  = "pool"
		poolLabels = []string{"pool", "profile", "root"}
	)

	labels := make(prometheus.Labels)
	labels["cluster"] = cluster

	return &PoolInfoCollector{
		conn:   conn,
		logger: logger,

		PGNum: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "pg_num",
				Help:        "The total count of PGs alotted to a pool",
				ConstLabels: labels,
			},
			poolLabels,
		),
		PlacementPGNum: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "pgp_num",
				Help:        "The total count of PGs alotted to a pool and used for placements",
				ConstLabels: labels,
			},
			poolLabels,
		),
		MinSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "min_size",
				Help:        "Minimum number of copies or chunks of an object that need to be present for active I/O",
				ConstLabels: labels,
			},
			poolLabels,
		),
		ActualSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "size",
				Help:        "Total copies or chunks of an object that need to be present for a healthy cluster",
				ConstLabels: labels,
			},
			poolLabels,
		),
		QuotaMaxBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "quota_max_bytes",
				Help:        "Maximum amount of bytes of data allowed in a pool",
				ConstLabels: labels,
			},
			poolLabels,
		),
		QuotaMaxObjects: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "quota_max_objects",
				Help:        "Maximum amount of RADOS objects allowed in a pool",
				ConstLabels: labels,
			},
			poolLabels,
		),
		StripeWidth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "stripe_width",
				Help:        "Stripe width of a RADOS object in a pool",
				ConstLabels: labels,
			},
			poolLabels,
		),
		ExpansionFactor: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Subsystem:   subSystem,
				Name:        "expansion_factor",
				Help:        "Data expansion multiplier for a pool",
				ConstLabels: labels,
			},
			poolLabels,
		),
	}
}

func (p *PoolInfoCollector) collectorList() []prometheus.Collector {
	return []prometheus.Collector{
		p.PGNum,
		p.PlacementPGNum,
		p.MinSize,
		p.ActualSize,
		p.QuotaMaxBytes,
		p.QuotaMaxObjects,
		p.StripeWidth,
		p.ExpansionFactor,
	}
}

type poolInfo struct {
	Name            string  `json:"pool_name"`
	ActualSize      float64 `json:"size"`
	MinSize         float64 `json:"min_size"`
	PGNum           float64 `json:"pg_num"`
	PlacementPGNum  float64 `json:"pg_placement_num"`
	QuotaMaxBytes   float64 `json:"quota_max_bytes"`
	QuotaMaxObjects float64 `json:"quota_max_objects"`
	Profile         string  `json:"erasure_code_profile"`
	Type            int64   `json:"type"`
	StripeWidth     float64 `json:"stripe_width"`
	CrushRule       int64   `json:"crush_rule"`
}

type cephPoolInfo struct {
	Pools []poolInfo
}

func (p *PoolInfoCollector) collect() error {
	cmd := p.cephInfoCommand()
	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		p.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	ruleToRootMappings := p.getCrushRuleToRootMappings()

	stats := &cephPoolInfo{}
	if err := json.Unmarshal(buf, &stats.Pools); err != nil {
		return err
	}

	// Reset pool specfic metrics, pools can be removed
	p.PGNum.Reset()
	p.PlacementPGNum.Reset()
	p.MinSize.Reset()
	p.ActualSize.Reset()
	p.QuotaMaxBytes.Reset()
	p.QuotaMaxObjects.Reset()
	p.StripeWidth.Reset()
	p.ExpansionFactor.Reset()

	for _, pool := range stats.Pools {
		if pool.Type == poolReplicated {
			pool.Profile = "replicated"
		}
		labelValues := []string{pool.Name, pool.Profile, ruleToRootMappings[pool.CrushRule]}
		p.PGNum.WithLabelValues(labelValues...).Set(pool.PGNum)
		p.PlacementPGNum.WithLabelValues(labelValues...).Set(pool.PlacementPGNum)
		p.MinSize.WithLabelValues(labelValues...).Set(pool.MinSize)
		p.ActualSize.WithLabelValues(labelValues...).Set(pool.ActualSize)
		p.QuotaMaxBytes.WithLabelValues(labelValues...).Set(pool.QuotaMaxBytes)
		p.QuotaMaxObjects.WithLabelValues(labelValues...).Set(pool.QuotaMaxObjects)
		p.StripeWidth.WithLabelValues(labelValues...).Set(pool.StripeWidth)
		p.ExpansionFactor.WithLabelValues(labelValues...).Set(p.getExpansionFactor(pool))
	}

	return nil
}

func (p *PoolInfoCollector) cephInfoCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd pool ls",
		"detail": "detail",
		"format": "json",
	})
	if err != nil {
		p.logger.WithError(err).Panic("error marshalling ceph osd pool ls")
	}
	return cmd
}

// Describe fulfills the prometheus.Collector's interface and sends the descriptors
// of pool's metrics to the given channel.
func (p *PoolInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range p.collectorList() {
		metric.Describe(ch)
	}
}

// Collect extracts the current values of all the metrics and sends them to the
// prometheus channel.
func (p *PoolInfoCollector) Collect(ch chan<- prometheus.Metric) {
	p.logger.Debug("collecting pool metrics")
	if err := p.collect(); err != nil {
		p.logger.WithError(err).Error("error collecting pool metrics")
		return
	}

	for _, metric := range p.collectorList() {
		metric.Collect(ch)
	}
}

func (p *PoolInfoCollector) getExpansionFactor(pool poolInfo) float64 {
	if ef, ok := p.getECExpansionFactor(pool); ok {
		return ef
	}
	// Non-EC pool (or unable to get profile info); assume that it's replicated.
	return pool.ActualSize
}

func (p *PoolInfoCollector) getECExpansionFactor(pool poolInfo) (float64, bool) {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd erasure-code-profile get",
		"name":   pool.Profile,
		"format": "json",
	})

	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		return -1, false
	}

	type ecInfo struct {
		K string `json:"k"`
		M string `json:"m"`
	}

	ecStats := ecInfo{}
	err = json.Unmarshal(buf, &ecStats)
	if err != nil || ecStats.K == "" || ecStats.M == "" {
		return -1, false
	}

	k, _ := strconv.ParseFloat(ecStats.K, 64)
	m, _ := strconv.ParseFloat(ecStats.M, 64)

	expansionFactor := (k + m) / k
	roundedExpansion := math.Round(expansionFactor*100) / 100
	return roundedExpansion, true
}

func (p *PoolInfoCollector) getCrushRuleToRootMappings() map[int64]string {
	mappings := make(map[int64]string)

	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd crush rule dump",
		"format": "json",
	})
	if err != nil {
		p.logger.WithError(err).Panic("error marshalling ceph osd crush rule dump")
	}

	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		p.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return mappings
	}

	var rules []struct {
		RuleID int64 `json:"rule_id"`
		Steps  []struct {
			ItemName string `json:"item_name"`
			Op       string `json:"op"`
		} `json:"steps"`
	}

	err = json.Unmarshal(buf, &rules)
	if err != nil {
		p.logger.WithError(err).Error("error unmarshalling crush rules")

		return mappings
	}

	for _, rule := range rules {
		if len(rule.Steps) == 0 {
			continue
		}
		for _, step := range rule.Steps {
			// Although there can be multiple "take" steps, there
			// usually aren't in practice. The "take" item isn't
			// necessarily a crush root, but assuming so is good
			// enough for most cases.
			if step.Op == "take" {
				mappings[rule.RuleID] = step.ItemName
			}
		}
	}

	return mappings
}
