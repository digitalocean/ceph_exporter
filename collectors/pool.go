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
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	poolReplicated = 1
	poolErasure    = 3
)

// PoolInfoCollector gives information about each pool that exists in a given
// ceph cluster.
type PoolInfoCollector struct {
	conn Conn

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
func NewPoolInfoCollector(conn Conn, cluster string) *PoolInfoCollector {
	var (
		subSystem  = "pool"
		poolLabels = []string{"pool", "profile"}
	)

	labels := make(prometheus.Labels)
	labels["cluster"] = cluster

	return &PoolInfoCollector{
		conn: conn,

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
	StripeWidth     float64 `json:"stripe_width"`
}

type cephPoolInfo struct {
	Pools []poolInfo
}

func (p *PoolInfoCollector) collect() error {
	cmd := p.cephInfoCommand()
	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

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
		p.PGNum.WithLabelValues(pool.Name, pool.Profile).Set(pool.PGNum)
		p.PlacementPGNum.WithLabelValues(pool.Name, pool.Profile).Set(pool.PlacementPGNum)
		p.MinSize.WithLabelValues(pool.Name, pool.Profile).Set(pool.MinSize)
		p.ActualSize.WithLabelValues(pool.Name, pool.Profile).Set(pool.ActualSize)
		p.QuotaMaxBytes.WithLabelValues(pool.Name, pool.Profile).Set(pool.QuotaMaxBytes)
		p.QuotaMaxObjects.WithLabelValues(pool.Name, pool.Profile).Set(pool.QuotaMaxObjects)
		p.StripeWidth.WithLabelValues(pool.Name, pool.Profile).Set(pool.StripeWidth)
		p.ExpansionFactor.WithLabelValues(pool.Name, pool.Profile).Set(p.getExpansionCommand(pool))
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
		// panic! because ideally in no world this hard-coded input
		// should fail.
		panic(err)
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
	if err := p.collect(); err != nil {
		log.Println("[ERROR] failed collecting pool usage metrics:", err)
		return
	}

	for _, metric := range p.collectorList() {
		metric.Collect(ch)
	}
}

func (p *PoolInfoCollector) getExpansionCommand(pool poolInfo) float64 {
	prefix := fmt.Sprintf("osd erasure-code-profile get %s", pool.Profile)
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": prefix,
		"detail": "detail",
		"format": "json",
	})

	buf, _, err := p.conn.MonCommand(cmd)
	if err != nil {
		return -1
	}

	type ecInfo struct {
		K string `json:"k"`
		M string `json:"m"`
	}

	ecStats := ecInfo{}
	err = json.Unmarshal(buf, &ecStats)
	if err != nil || ecStats.K == "" || ecStats.M == "" {
		return pool.ActualSize
	}

	k, _ := strconv.ParseFloat(ecStats.K, 64)
	m, _ := strconv.ParseFloat(ecStats.M, 64)

	expansionFactor := (k + m) / k
	roundedExpansion := math.Round(expansionFactor*100) / 100
	return roundedExpansion
}
