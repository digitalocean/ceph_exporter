//   Copyright 2016 DigitalOcean
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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	recoveryIORateRegex    = regexp.MustCompile(`(\d+) (\w{2})/s`)
	recoveryIOKeysRegex    = regexp.MustCompile(`(\d+) keys/s`)
	recoveryIOObjectsRegex = regexp.MustCompile(`(\d+) objects/s`)
	clientIOReadRegex      = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s rd`)
	clientIOWriteRegex     = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s wr`)
	clientIOReadOpsRegex   = regexp.MustCompile(`(\d+) op/s rd`)
	clientIOWriteOpsRegex  = regexp.MustCompile(`(\d+) op/s wr`)
	cacheFlushRateRegex    = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s flush`)
	cacheEvictRateRegex    = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s evict`)
	cachePromoteOpsRegex   = regexp.MustCompile(`(\d+) op/s promote`)

	// Older versions of Ceph, hammer (v0.94) and below, support this format.
	clientIOOpsRegex = regexp.MustCompile(`(\d+) op/s[^ \w]*$`)
)

// ClusterHealthCollector collects information about the health of an overall cluster.
// It surfaces changes in the ceph parameters unlike data usage that ClusterUsageCollector
// does.
type ClusterHealthCollector struct {
	// conn holds connection to the Ceph cluster
	conn Conn

	// HealthStatus shows the overall health status of a given cluster.
	HealthStatus prometheus.Gauge

	// TotalPGs shows the total no. of PGs the cluster constitutes of.
	TotalPGs prometheus.Gauge

	// DegradedPGs shows the no. of PGs that have some of the replicas
	// missing.
	DegradedPGs prometheus.Gauge

	// StuckDegradedPGs shows the no. of PGs that have some of the replicas
	// missing, and are stuck in that state.
	StuckDegradedPGs prometheus.Gauge

	// UncleanPGs shows the no. of PGs that do not have all objects in the PG
	// that are supposed to be in it.
	UncleanPGs prometheus.Gauge

	// StuckUncleanPGs shows the no. of PGs that do not have all objects in the PG
	// that are supposed to be in it, and are stuck in that state.
	StuckUncleanPGs prometheus.Gauge

	// UndersizedPGs depicts the no. of PGs that have fewer copies than configured
	// replication level.
	UndersizedPGs prometheus.Gauge

	// StuckUndersizedPGs depicts the no. of PGs that have fewer copies than configured
	// replication level, and are stuck in that state.
	StuckUndersizedPGs prometheus.Gauge

	// StalePGs depicts no. of PGs that are in an unknown state i.e. monitors do not know
	// anything about their latest state since their pg mapping was modified.
	StalePGs prometheus.Gauge

	// StuckStalePGs depicts no. of PGs that are in an unknown state i.e. monitors do not know
	// anything about their latest state since their pg mapping was modified, and are stuck
	// in that state.
	StuckStalePGs prometheus.Gauge

	// ScrubbingPGs depicts no. of PGs that are in scrubbing state.
	// Light scrubbing checks the object size and attributes.
	ScrubbingPGs prometheus.Gauge

	// DeepScrubbingPGs depicts no. of PGs that are in scrubbing+deep state.
	// Deep scrubbing reads the data and uses checksums to ensure data integrity.
	DeepScrubbingPGs prometheus.Gauge

	// SlowRequests depicts no. of total slow requests in the cluster
	SlowRequests prometheus.Gauge

	// DegradedObjectsCount gives the no. of RADOS objects are constitute the degraded PGs.
	// This includes object replicas in its count.
	DegradedObjectsCount prometheus.Gauge

	// MisplacedObjectsCount gives the no. of RADOS objects that constitute the misplaced PGs.
	// Misplaced PGs usually represent the PGs that are not in the storage locations that
	// they should be in. This is different than degraded PGs which means a PG has fewer copies
	// that it should.
	// This includes object replicas in its count.
	MisplacedObjectsCount prometheus.Gauge

	// OSDsDown show the no. of OSDs that are in the DOWN state.
	OSDsDown prometheus.Gauge

	// OSDsUp show the no. of OSDs that are in the UP state and are able to serve requests.
	OSDsUp prometheus.Gauge

	// OSDsIn shows the no. of OSDs that are marked as IN in the cluster.
	OSDsIn prometheus.Gauge

	// OSDsNum shows the no. of total OSDs the cluster has.
	OSDsNum prometheus.Gauge

	// RemappedPGs show the no. of PGs that are currently remapped and needs to be moved
	// to newer OSDs.
	RemappedPGs prometheus.Gauge

	// RecoveryIORate shows the i/o rate at which the cluster is performing its ongoing
	// recovery at.
	RecoveryIORate prometheus.Gauge

	// RecoveryIOKeys shows the rate of rados keys recovery.
	RecoveryIOKeys prometheus.Gauge

	// RecoveryIOObjects shows the rate of rados objects being recovered.
	RecoveryIOObjects prometheus.Gauge

	// ClientIORead shows the total client read i/o on the cluster.
	ClientIORead prometheus.Gauge

	// ClientIOWrite shows the total client write i/o on the cluster.
	ClientIOWrite prometheus.Gauge

	// ClientIOOps shows the rate of total operations conducted by all clients on the cluster.
	ClientIOOps prometheus.Gauge

	// ClientIOReadOps shows the rate of total read operations conducted by all clients on the cluster.
	ClientIOReadOps prometheus.Gauge

	// ClientIOWriteOps shows the rate of total write operations conducted by all clients on the cluster.
	ClientIOWriteOps prometheus.Gauge

	// CacheFlushIORate shows the i/o rate at which data is being flushed from the cache pool.
	CacheFlushIORate prometheus.Gauge

	// CacheEvictIORate shows the i/o rate at which data is being flushed from the cache pool.
	CacheEvictIORate prometheus.Gauge

	// CachePromoteIOOps shows the rate of operations promoting objects to the cache pool.
	CachePromoteIOOps prometheus.Gauge
}

const (
	// CephHealthOK denotes the status of ceph cluster when healthy.
	CephHealthOK = "HEALTH_OK"

	// CephHealthWarn denotes the status of ceph cluster when unhealthy but recovering.
	CephHealthWarn = "HEALTH_WARN"

	// CephHealthErr denotes the status of ceph cluster when unhealthy but usually needs
	// manual intervention.
	CephHealthErr = "HEALTH_ERR"
)

// NewClusterHealthCollector creates a new instance of ClusterHealthCollector to collect health
// metrics on.
func NewClusterHealthCollector(conn Conn, cluster string) *ClusterHealthCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = cluster

	return &ClusterHealthCollector{
		conn: conn,

		HealthStatus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "health_status",
				Help:        "Health status of Cluster, can vary only between 3 states (err:2, warn:1, ok:0)",
				ConstLabels: labels,
			},
		),
		TotalPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "total_pgs",
				Help:        "Total no. of PGs in the cluster",
				ConstLabels: labels,
			},
		),
		ScrubbingPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "scrubbing_pgs",
				Help:        "No. of scrubbing PGs in the cluster",
				ConstLabels: labels,
			},
		),
		DeepScrubbingPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "deep_scrubbing_pgs",
				Help:        "No. of deep scrubbing PGs in the cluster",
				ConstLabels: labels,
			},
		),
		SlowRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "slow_requests",
				Help:        "No. of slow requests",
				ConstLabels: labels,
			},
		),
		DegradedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "degraded_pgs",
				Help:        "No. of PGs in a degraded state",
				ConstLabels: labels,
			},
		),
		StuckDegradedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "stuck_degraded_pgs",
				Help:        "No. of PGs stuck in a degraded state",
				ConstLabels: labels,
			},
		),
		UncleanPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "unclean_pgs",
				Help:        "No. of PGs in an unclean state",
				ConstLabels: labels,
			},
		),
		StuckUncleanPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "stuck_unclean_pgs",
				Help:        "No. of PGs stuck in an unclean state",
				ConstLabels: labels,
			},
		),
		UndersizedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "undersized_pgs",
				Help:        "No. of undersized PGs in the cluster",
				ConstLabels: labels,
			},
		),
		StuckUndersizedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "stuck_undersized_pgs",
				Help:        "No. of stuck undersized PGs in the cluster",
				ConstLabels: labels,
			},
		),
		StalePGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "stale_pgs",
				Help:        "No. of stale PGs in the cluster",
				ConstLabels: labels,
			},
		),
		StuckStalePGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "stuck_stale_pgs",
				Help:        "No. of stuck stale PGs in the cluster",
				ConstLabels: labels,
			},
		),
		DegradedObjectsCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "degraded_objects",
				Help:        "No. of degraded objects across all PGs, includes replicas",
				ConstLabels: labels,
			},
		),
		MisplacedObjectsCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "misplaced_objects",
				Help:        "No. of misplaced objects across all PGs, includes replicas",
				ConstLabels: labels,
			},
		),
		OSDsDown: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osds_down",
				Help:        "Count of OSDs that are in DOWN state",
				ConstLabels: labels,
			},
		),
		OSDsUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osds_up",
				Help:        "Count of OSDs that are in UP state",
				ConstLabels: labels,
			},
		),
		OSDsIn: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osds_in",
				Help:        "Count of OSDs that are in IN state and available to serve requests",
				ConstLabels: labels,
			},
		),
		OSDsNum: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osds",
				Help:        "Count of total OSDs in the cluster",
				ConstLabels: labels,
			},
		),
		RemappedPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "pgs_remapped",
				Help:        "No. of PGs that are remapped and incurring cluster-wide movement",
				ConstLabels: labels,
			},
		),
		RecoveryIORate: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "recovery_io_bytes",
				Help:        "Rate of bytes being recovered in cluster per second",
				ConstLabels: labels,
			},
		),
		RecoveryIOKeys: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "recovery_io_keys",
				Help:        "Rate of keys being recovered in cluster per second",
				ConstLabels: labels,
			},
		),
		RecoveryIOObjects: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "recovery_io_objects",
				Help:        "Rate of objects being recovered in cluster per second",
				ConstLabels: labels,
			},
		),
		ClientIORead: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "client_io_read_bytes",
				Help:        "Rate of bytes being read by all clients per second",
				ConstLabels: labels,
			},
		),
		ClientIOWrite: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "client_io_write_bytes",
				Help:        "Rate of bytes being written by all clients per second",
				ConstLabels: labels,
			},
		),
		ClientIOOps: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "client_io_ops",
				Help:        "Total client ops on the cluster measured per second",
				ConstLabels: labels,
			},
		),
		ClientIOReadOps: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "client_io_read_ops",
				Help:        "Total client read I/O ops on the cluster measured per second",
				ConstLabels: labels,
			},
		),
		ClientIOWriteOps: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "client_io_write_ops",
				Help:        "Total client write I/O ops on the cluster measured per second",
				ConstLabels: labels,
			},
		),
		CacheFlushIORate: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "cache_flush_io_bytes",
				Help:        "Rate of bytes being flushed from the cache pool per second",
				ConstLabels: labels,
			},
		),
		CacheEvictIORate: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "cache_evict_io_bytes",
				Help:        "Rate of bytes being evicted from the cache pool per second",
				ConstLabels: labels,
			},
		),
		CachePromoteIOOps: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "cache_promote_io_ops",
				Help:        "Total cache promote operations measured per second",
				ConstLabels: labels,
			},
		),
	}
}

func (c *ClusterHealthCollector) metricsList() []prometheus.Metric {
	return []prometheus.Metric{
		c.HealthStatus,
		c.TotalPGs,
		c.DegradedPGs,
		c.StuckDegradedPGs,
		c.UncleanPGs,
		c.StuckUncleanPGs,
		c.UndersizedPGs,
		c.StuckUndersizedPGs,
		c.StalePGs,
		c.StuckStalePGs,
		c.ScrubbingPGs,
		c.DeepScrubbingPGs,
		c.SlowRequests,
		c.DegradedObjectsCount,
		c.MisplacedObjectsCount,
		c.OSDsDown,
		c.OSDsUp,
		c.OSDsIn,
		c.OSDsNum,
		c.RemappedPGs,
		c.RecoveryIORate,
		c.RecoveryIOKeys,
		c.RecoveryIOObjects,
		c.ClientIORead,
		c.ClientIOWrite,
		c.ClientIOOps,
		c.ClientIOReadOps,
		c.ClientIOWriteOps,
		c.CacheFlushIORate,
		c.CacheEvictIORate,
		c.CachePromoteIOOps,
	}
}

type cephHealthStats struct {
	Health struct {
		Summary []struct {
			Severity string `json:"severity"`
			Summary  string `json:"summary"`
		} `json:"summary"`
		OverallStatus string `json:"status"`
		Checks        map[string]struct {
			Severity string `json:"severity"`
			Summary  struct {
				Message string `json:"message"`
			} `json:"summary"`
		} `json:"checks"`
	} `json:"health"`
	OSDMap struct {
		OSDMap struct {
			NumOSDs        json.Number `json:"num_osds"`
			NumUpOSDs      json.Number `json:"num_up_osds"`
			NumInOSDs      json.Number `json:"num_in_osds"`
			NumRemappedPGs json.Number `json:"num_remapped_pgs"`
		} `json:"osdmap"`
	} `json:"osdmap"`
	PGMap struct {
		PGsByState []struct {
			StateName string      `json:"state_name"`
			Count     json.Number `json:"count"`
		} `json:"pgs_by_state"`
		NumPGs json.Number `json:"num_pgs"`
	} `json:"pgmap"`
}

func (c *ClusterHealthCollector) collect() error {
	cmd := c.cephJSONUsage()
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

	stats := &cephHealthStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	for _, metric := range c.metricsList() {
		if gauge, ok := metric.(prometheus.Gauge); ok {
			gauge.Set(0)
		}
	}

	switch stats.Health.OverallStatus {
	case CephHealthOK:
		c.HealthStatus.Set(0)
	case CephHealthWarn:
		c.HealthStatus.Set(1)
	case CephHealthErr:
		c.HealthStatus.Set(2)
	default:
		c.HealthStatus.Set(2)
	}

	var (
		degradedRegex            = regexp.MustCompile(`([\d]+) pgs degraded`)
		stuckDegradedRegex       = regexp.MustCompile(`([\d]+) pgs stuck degraded`)
		uncleanRegex             = regexp.MustCompile(`([\d]+) pgs unclean`)
		stuckUncleanRegex        = regexp.MustCompile(`([\d]+) pgs stuck unclean`)
		undersizedRegex          = regexp.MustCompile(`([\d]+) pgs undersized`)
		stuckUndersizedRegex     = regexp.MustCompile(`([\d]+) pgs stuck undersized`)
		staleRegex               = regexp.MustCompile(`([\d]+) pgs stale`)
		stuckStaleRegex          = regexp.MustCompile(`([\d]+) pgs stuck stale`)
		slowRequestRegex         = regexp.MustCompile(`([\d]+) requests are blocked`)
		slowRequestRegexLuminous = regexp.MustCompile(`([\d]+) slow requests are blocked`)
		degradedObjectsRegex     = regexp.MustCompile(`recovery ([\d]+)/([\d]+) objects degraded`)
		misplacedObjectsRegex    = regexp.MustCompile(`recovery ([\d]+)/([\d]+) objects misplaced`)
	)

	for _, s := range stats.Health.Summary {
		matched := degradedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.DegradedPGs.Set(float64(v))
		}

		matched = stuckDegradedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.StuckDegradedPGs.Set(float64(v))
		}

		matched = uncleanRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.UncleanPGs.Set(float64(v))
		}

		matched = stuckUncleanRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.StuckUncleanPGs.Set(float64(v))
		}

		matched = undersizedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.UndersizedPGs.Set(float64(v))
		}

		matched = stuckUndersizedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.StuckUndersizedPGs.Set(float64(v))
		}

		matched = staleRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.StalePGs.Set(float64(v))
		}

		matched = stuckStaleRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.StuckStalePGs.Set(float64(v))
		}

		matched = slowRequestRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.SlowRequests.Set(float64(v))
		}

		matched = degradedObjectsRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 3 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.DegradedObjectsCount.Set(float64(v))
		}

		matched = misplacedObjectsRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 3 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			c.MisplacedObjectsCount.Set(float64(v))
		}
	}

	for k, check := range stats.Health.Checks {
		if k == "REQUEST_SLOW" {
			matched := slowRequestRegexLuminous.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 2 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				c.SlowRequests.Set(float64(v))
			}
		}
	}

	osdsUp, err := stats.OSDMap.OSDMap.NumUpOSDs.Float64()
	if err != nil {
		return err
	}
	c.OSDsUp.Set(osdsUp)

	osdsIn, err := stats.OSDMap.OSDMap.NumInOSDs.Float64()
	if err != nil {
		return err
	}
	c.OSDsIn.Set(osdsIn)

	osdsNum, err := stats.OSDMap.OSDMap.NumOSDs.Float64()
	if err != nil {
		return err
	}
	c.OSDsNum.Set(osdsNum)

	// Ceph (until v10.2.3) doesn't expose the value of down OSDs
	// from its status, which is why we have to compute it ourselves.
	c.OSDsDown.Set(osdsNum - osdsUp)

	remappedPGs, err := stats.OSDMap.OSDMap.NumRemappedPGs.Float64()
	if err != nil {
		return err
	}
	c.RemappedPGs.Set(remappedPGs)

	scrubbingPGs := float64(0)
	deepScrubbingPGs := float64(0)
	for _, pg := range stats.PGMap.PGsByState {
		if strings.Contains(pg.StateName, "scrubbing") {
			if strings.Contains(pg.StateName, "deep") {
				deepScrubbingCount, err := pg.Count.Float64()
				if err != nil {
					return err
				}
				deepScrubbingPGs += deepScrubbingCount
			} else {
				scrubbingCount, err := pg.Count.Float64()
				if err != nil {
					return err
				}
				scrubbingPGs += scrubbingCount
			}
		}
	}
	c.ScrubbingPGs.Set(scrubbingPGs)
	c.DeepScrubbingPGs.Set(deepScrubbingPGs)

	totalPGs, err := stats.PGMap.NumPGs.Float64()
	if err != nil {
		return err
	}
	c.TotalPGs.Set(totalPGs)

	return nil
}

type format string

const (
	jsonFormat  format = "json"
	plainFormat format = "plain"
)

func (c *ClusterHealthCollector) cephPlainUsage() []byte {
	return c.cephUsageCommand(plainFormat)
}

func (c *ClusterHealthCollector) cephJSONUsage() []byte {
	return c.cephUsageCommand(jsonFormat)
}

func (c *ClusterHealthCollector) cephUsageCommand(f format) []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "status",
		"format": f,
	})
	if err != nil {
		// panic! because ideally in no world this hard-coded input
		// should fail.
		panic(err)
	}
	return cmd
}

func (c *ClusterHealthCollector) collectRecoveryClientIO() error {
	cmd := c.cephPlainUsage()
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(bytes.NewReader(buf))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		switch {
		case strings.HasPrefix(line, "recovery io"):
			if err := c.collectRecoveryIO(line); err != nil {
				return err
			}
		case strings.HasPrefix(line, "recovery:"):
			if err := c.collectRecoveryIO(line); err != nil {
				return err
			}
		case strings.HasPrefix(line, "client io"):
			if err := c.collectClientIO(line); err != nil {
				return err
			}
		case strings.HasPrefix(line, "client:"):
			if err := c.collectClientIO(line); err != nil {
				return err
			}
		case strings.HasPrefix(line, "cache io"):
			if err := c.collectCacheIO(line); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *ClusterHealthCollector) collectClientIO(clientStr string) error {
	matched := clientIOReadRegex.FindStringSubmatch(clientStr)
	if len(matched) == 3 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		switch strings.ToLower(matched[2]) {
		case "gb":
			v = v * 1e9
		case "mb":
			v = v * 1e6
		case "kb":
			v = v * 1e3
		default:
			return fmt.Errorf("can't parse units %q", matched[2])
		}

		c.ClientIORead.Set(float64(v))
	}

	matched = clientIOWriteRegex.FindStringSubmatch(clientStr)
	if len(matched) == 3 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		switch strings.ToLower(matched[2]) {
		case "gb":
			v = v * 1e9
		case "mb":
			v = v * 1e6
		case "kb":
			v = v * 1e3
		default:
			return fmt.Errorf("can't parse units %q", matched[2])
		}

		c.ClientIOWrite.Set(float64(v))
	}

	var clientIOOps float64
	matched = clientIOOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		clientIOOps = float64(v)
	}

	var clientIOReadOps, clientIOWriteOps float64
	matched = clientIOReadOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		clientIOReadOps = float64(v)
		c.ClientIOReadOps.Set(clientIOReadOps)
	}

	matched = clientIOWriteOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		clientIOWriteOps = float64(v)
		c.ClientIOWriteOps.Set(clientIOWriteOps)
	}

	// In versions older than Jewel, we directly get access to total
	// client I/O. But in Jewel and newer the format is changed to
	// separately display read and write IOPs. In such a case, we
	// compute and set the total IOPs ourselves.
	if clientIOOps == 0 {
		clientIOOps = clientIOReadOps + clientIOWriteOps
	}

	c.ClientIOOps.Set(clientIOOps)

	return nil
}

func (c *ClusterHealthCollector) collectRecoveryIO(recoveryStr string) error {
	matched := recoveryIORateRegex.FindStringSubmatch(recoveryStr)
	if len(matched) == 3 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		switch strings.ToLower(matched[2]) {
		case "gb":
			v = v * 1e9
		case "mb":
			v = v * 1e6
		case "kb":
			v = v * 1e3
		default:
			return fmt.Errorf("can't parse units %q", matched[2])
		}

		c.RecoveryIORate.Set(float64(v))
	}

	matched = recoveryIOKeysRegex.FindStringSubmatch(recoveryStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		c.RecoveryIOKeys.Set(float64(v))
	}

	matched = recoveryIOObjectsRegex.FindStringSubmatch(recoveryStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		c.RecoveryIOObjects.Set(float64(v))
	}
	return nil
}

func (c *ClusterHealthCollector) collectCacheIO(clientStr string) error {
	matched := cacheFlushRateRegex.FindStringSubmatch(clientStr)
	if len(matched) == 3 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		switch strings.ToLower(matched[2]) {
		case "gb":
			v = v * 1e9
		case "mb":
			v = v * 1e6
		case "kb":
			v = v * 1e3
		default:
			return fmt.Errorf("can't parse units %q", matched[2])
		}

		c.CacheFlushIORate.Set(float64(v))
	}

	matched = cacheEvictRateRegex.FindStringSubmatch(clientStr)
	if len(matched) == 3 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		switch strings.ToLower(matched[2]) {
		case "gb":
			v = v * 1e9
		case "mb":
			v = v * 1e6
		case "kb":
			v = v * 1e3
		default:
			return fmt.Errorf("can't parse units %q", matched[2])
		}

		c.CacheEvictIORate.Set(float64(v))
	}

	matched = cachePromoteOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		c.CachePromoteIOOps.Set(float64(v))
	}
	return nil
}

// Describe sends all the descriptions of individual metrics of ClusterHealthCollector
// to the provided prometheus channel.
func (c *ClusterHealthCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metricsList() {
		ch <- metric.Desc()
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (c *ClusterHealthCollector) Collect(ch chan<- prometheus.Metric) {
	if err := c.collect(); err != nil {
		log.Println("failed collecting cluster health metrics:", err)
	}

	if err := c.collectRecoveryClientIO(); err != nil {
		log.Println("failed collecting cluster recovery/client io:", err)
	}

	for _, metric := range c.metricsList() {
		ch <- metric
	}
}
