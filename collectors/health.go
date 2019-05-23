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
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	recoveryIORateRegex         = regexp.MustCompile(`(\d+) (\w{2})/s`)
	recoveryIOKeysRegex         = regexp.MustCompile(`(\d+) keys/s`)
	recoveryIOObjectsRegex      = regexp.MustCompile(`(\d+) objects/s`)
	clientReadBytesPerSecRegex  = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s rd`)
	clientWriteBytesPerSecRegex = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s wr`)
	clientIOReadOpsRegex        = regexp.MustCompile(`(\d+) op/s rd`)
	clientIOWriteOpsRegex       = regexp.MustCompile(`(\d+) op/s wr`)
	cacheFlushRateRegex         = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s flush`)
	cacheEvictRateRegex         = regexp.MustCompile(`(\d+) ([kKmMgG][bB])/s evict`)
	cachePromoteOpsRegex        = regexp.MustCompile(`(\d+) op/s promote`)

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

	// ActivePGs shows the no. of PGs the cluster is actively serving data
	// from.
	ActivePGs prometheus.Gauge

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

	// PeeringPGs depicts no. of PGs that have one or more OSDs undergo state changes
	// that need to be communicated to the remaining peers.
	PeeringPGs prometheus.Gauge

	// ScrubbingPGs depicts no. of PGs that are in scrubbing state.
	// Light scrubbing checks the object size and attributes.
	ScrubbingPGs prometheus.Gauge

	// DeepScrubbingPGs depicts no. of PGs that are in scrubbing+deep state.
	// Deep scrubbing reads the data and uses checksums to ensure data integrity.
	DeepScrubbingPGs prometheus.Gauge

	// RecoveringPGs depicts no. of PGs that are in recovering state.
	// The PGs in this state have been dequeued from recovery_wait queue and are
	// actively undergoing recovery.
	RecoveringPGs prometheus.Gauge

	// RecoveryWaitPGs depicts no. of PGs that are in recovery_wait state.
	// The PGs in this state are still in queue to start recovery on them.
	RecoveryWaitPGs prometheus.Gauge

	// BackfillingPGs depicts no. of PGs that are in backfilling state.
	// The PGs in this state have been dequeued from backfill_wait queue and are
	// actively undergoing recovery.
	BackfillingPGs prometheus.Gauge

	// BackfillWaitPGs depicts no. of PGs that are in backfill_wait state.
	// The PGs in this state are still in queue to start backfill on them.
	BackfillWaitPGs prometheus.Gauge

	// ForcedRecoveryPGs depicts no. of PGs that are undergoing forced recovery.
	ForcedRecoveryPGs prometheus.Gauge

	// ForcedBackfillPGs depicts no. of PGs that are undergoing forced backfill.
	ForcedBackfillPGs prometheus.Gauge

	// DownPGs depicts no. of PGs that are currently down and not able to serve traffic.
	DownPGs prometheus.Gauge

	// SlowRequests depicts no. of total slow requests in the cluster
	// This stat exists only for backwards compatbility.
	SlowRequests prometheus.Gauge

	// StuckRequests depicts no. of total requests in the cluster
	// that haven't been served for over an hour.
	StuckRequests prometheus.Gauge

	// SlowRequestsByOSDDesc depicts no. of total slow requests in the cluster
	// labelled by OSD.
	SlowRequestsByOSDDesc *prometheus.Desc

	// DegradedObjectsCount gives the no. of RADOS objects are constitute the degraded PGs.
	// This includes object replicas in its count.
	DegradedObjectsCount prometheus.Gauge

	// MisplacedObjectsCount gives the no. of RADOS objects that constitute the misplaced PGs.
	// Misplaced PGs usually represent the PGs that are not in the storage locations that
	// they should be in. This is different than degraded PGs which means a PG has fewer copies
	// that it should.
	// This includes object replicas in its count.
	MisplacedObjectsCount prometheus.Gauge

	// OSDMapFlags
	OSDMapFlagFull        prometheus.Gauge
	OSDMapFlagPauseRd     prometheus.Gauge
	OSDMapFlagPauseWr     prometheus.Gauge
	OSDMapFlagNoUp        prometheus.Gauge
	OSDMapFlagNoDown      prometheus.Gauge
	OSDMapFlagNoIn        prometheus.Gauge
	OSDMapFlagNoOut       prometheus.Gauge
	OSDMapFlagNoBackfill  prometheus.Gauge
	OSDMapFlagNoRecover   prometheus.Gauge
	OSDMapFlagNoRebalance prometheus.Gauge
	OSDMapFlagNoScrub     prometheus.Gauge
	OSDMapFlagNoDeepScrub prometheus.Gauge
	OSDMapFlagNoTierAgent prometheus.Gauge

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

	// ClientReadBytesPerSec shows the total client read i/o on the cluster.
	ClientReadBytesPerSec prometheus.Gauge

	// ClientWriteBytesPerSec shows the total client write i/o on the cluster.
	ClientWriteBytesPerSec prometheus.Gauge

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
		ActivePGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "active_pgs",
				Help:        "No. of active PGs in the cluster",
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
		RecoveringPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "recovering_pgs",
				Help:        "No. of recovering PGs in the cluster",
				ConstLabels: labels,
			},
		),
		RecoveryWaitPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "recovery_wait_pgs",
				Help:        "No. of PGs in the cluster with recovery_wait state",
				ConstLabels: labels,
			},
		),
		BackfillingPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "backfilling_pgs",
				Help:        "No. of backfilling PGs in the cluster",
				ConstLabels: labels,
			},
		),
		BackfillWaitPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "backfill_wait_pgs",
				Help:        "No. of PGs in the cluster with backfill_wait state",
				ConstLabels: labels,
			},
		),
		ForcedRecoveryPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "forced_recovery_pgs",
				Help:        "No. of PGs in the cluster with forced_recovery state",
				ConstLabels: labels,
			},
		),
		ForcedBackfillPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "forced_backfill_pgs",
				Help:        "No. of PGs in the cluster with forced_backfill state",
				ConstLabels: labels,
			},
		),
		DownPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "down_pgs",
				Help:        "No. of PGs in the cluster in down state",
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
		StuckRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "stuck_requests",
				Help:        "No. of stuck requests",
				ConstLabels: labels,
			},
		),
		SlowRequestsByOSDDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_slow_requests_osd", cephNamespace),
			"No. of slow requests per OSD",
			[]string{"osd"},
			labels,
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
		PeeringPGs: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "peering_pgs",
				Help:        "No. of peering PGs in the cluster",
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
		OSDMapFlagFull: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_full",
				Help:        "The cluster is flagged as full and cannot service writes",
				ConstLabels: labels,
			},
		),
		OSDMapFlagPauseRd: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_pauserd",
				Help:        "Reads are paused",
				ConstLabels: labels,
			},
		),
		OSDMapFlagPauseWr: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_pausewr",
				Help:        "Writes are paused",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_noup",
				Help:        "OSDs are not allowed to start",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoDown: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_nodown",
				Help:        "OSD failure reports are ignored, OSDs will not be marked as down",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoIn: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_noin",
				Help:        "OSDs that are out will not be automatically marked in",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoOut: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_noout",
				Help:        "OSDs will not be automatically marked out after the configured interval",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoBackfill: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_nobackfill",
				Help:        "OSDs will not be backfilled",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoRecover: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_norecover",
				Help:        "Recovery is suspended",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoRebalance: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_norebalance",
				Help:        "Data rebalancing is suspended",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoScrub: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_noscrub",
				Help:        "Scrubbing is disabled",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoDeepScrub: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_nodeep_scrub",
				Help:        "Deep scrubbing is disabled",
				ConstLabels: labels,
			},
		),
		OSDMapFlagNoTierAgent: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "osdmap_flag_notieragent",
				Help:        "Cache tiering activity is suspended",
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
		ClientReadBytesPerSec: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "client_io_read_bytes",
				Help:        "Rate of bytes being read by all clients per second",
				ConstLabels: labels,
			},
		),
		ClientWriteBytesPerSec: prometheus.NewGauge(
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
		c.ActivePGs,
		c.StuckDegradedPGs,
		c.UncleanPGs,
		c.StuckUncleanPGs,
		c.UndersizedPGs,
		c.StuckUndersizedPGs,
		c.StalePGs,
		c.StuckStalePGs,
		c.PeeringPGs,
		c.ScrubbingPGs,
		c.DeepScrubbingPGs,
		c.RecoveringPGs,
		c.RecoveryWaitPGs,
		c.BackfillingPGs,
		c.BackfillWaitPGs,
		c.ForcedRecoveryPGs,
		c.ForcedBackfillPGs,
		c.DownPGs,
		c.SlowRequests,
		c.StuckRequests,
		c.DegradedObjectsCount,
		c.MisplacedObjectsCount,
		c.OSDMapFlagFull,
		c.OSDMapFlagPauseRd,
		c.OSDMapFlagPauseWr,
		c.OSDMapFlagNoUp,
		c.OSDMapFlagNoDown,
		c.OSDMapFlagNoIn,
		c.OSDMapFlagNoOut,
		c.OSDMapFlagNoBackfill,
		c.OSDMapFlagNoRecover,
		c.OSDMapFlagNoRebalance,
		c.OSDMapFlagNoScrub,
		c.OSDMapFlagNoDeepScrub,
		c.OSDMapFlagNoTierAgent,
		c.OSDsDown,
		c.OSDsUp,
		c.OSDsIn,
		c.OSDsNum,
		c.RemappedPGs,
		c.RecoveryIORate,
		c.RecoveryIOKeys,
		c.RecoveryIOObjects,
		c.ClientReadBytesPerSec,
		c.ClientWriteBytesPerSec,
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
		OverallStatus string `json:"overall_status"`
		Status        string `json:"status"`
		Checks        map[string]struct {
			Severity string `json:"severity"`
			Summary  struct {
				Message string `json:"message"`
			} `json:"summary"`
		} `json:"checks"`
	} `json:"health"`
	OSDMap struct {
		OSDMap struct {
			NumOSDs        float64 `json:"num_osds"`
			NumUpOSDs      float64 `json:"num_up_osds"`
			NumInOSDs      float64 `json:"num_in_osds"`
			NumRemappedPGs float64 `json:"num_remapped_pgs"`
		} `json:"osdmap"`
	} `json:"osdmap"`
	PGMap struct {
		NumPGs                  float64 `json:"num_pgs"`
		WriteOpPerSec           float64 `json:"write_op_per_sec"`
		ReadOpPerSec            float64 `json:"read_op_per_sec"`
		WriteBytePerSec         float64 `json:"write_bytes_sec"`
		ReadBytePerSec          float64 `json:"read_bytes_sec"`
		RecoveringObjectsPerSec float64 `json:"recovering_objects_per_sec"`
		RecoveringBytePerSec    float64 `json:"recovering_bytes_per_sec"`
		RecoveringKeysPerSec    float64 `json:"recovering_keys_per_sec"`
		CacheFlushBytePerSec    float64 `json:"flush_bytes_sec"`
		CacheEvictBytePerSec    float64 `json:"evict_bytes_sec"`
		CachePromoteOpPerSec    float64 `json:"promote_op_per_sec"`
		DegradedObjects         float64 `json:"degraded_objects"`
		MisplacedObjects        float64 `json:"misplaced_objects"`
		PGsByState              []struct {
			Count  float64 `json:"count"`
			States string  `json:"state_name"`
		} `json:"pgs_by_state"`
	} `json:"pgmap"`
}

type cephHealthDetailStats struct {
	Checks map[string]struct {
		Details []struct {
			Message string `json:"message"`
		} `json:"detail"`
		Summary struct {
			Message string `json:"message"`
		} `json:"summary"`
		Severity string `json:"severity"`
	} `json:"checks"`
}

func (c *ClusterHealthCollector) collect(ch chan<- prometheus.Metric) error {
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

	// This will be set only if Luminous is running. Will be
	// ignored otherwise.
	switch stats.Health.Status {
	case CephHealthOK:
		c.HealthStatus.Set(0)
	case CephHealthWarn:
		c.HealthStatus.Set(1)
	case CephHealthErr:
		c.HealthStatus.Set(2)
	}

	var (
		degradedRegex             = regexp.MustCompile(`([\d]+) pgs degraded`)
		stuckDegradedRegex        = regexp.MustCompile(`([\d]+) pgs stuck degraded`)
		uncleanRegex              = regexp.MustCompile(`([\d]+) pgs unclean`)
		stuckUncleanRegex         = regexp.MustCompile(`([\d]+) pgs stuck unclean`)
		undersizedRegex           = regexp.MustCompile(`([\d]+) pgs undersized`)
		stuckUndersizedRegex      = regexp.MustCompile(`([\d]+) pgs stuck undersized`)
		staleRegex                = regexp.MustCompile(`([\d]+) pgs stale`)
		stuckStaleRegex           = regexp.MustCompile(`([\d]+) pgs stuck stale`)
		slowRequestRegex          = regexp.MustCompile(`([\d]+) requests are blocked`)
		slowRequestRegexLuminous  = regexp.MustCompile(`([\d]+) slow requests are blocked`)
		stuckRequestRegexLuminous = regexp.MustCompile(`([\d]+) stuck requests are blocked`)
		degradedObjectsRegex      = regexp.MustCompile(`([\d]+)/([\d]+) objects degraded`)
		misplacedObjectsRegex     = regexp.MustCompile(`([\d]+)/([\d]+) objects misplaced`)
		osdmapFlagsRegex          = regexp.MustCompile(`([^ ]+) flag\(s\) set`)
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

		if k == "REQUEST_STUCK" {
			matched := stuckRequestRegexLuminous.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 2 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				c.StuckRequests.Set(float64(v))
			}
		}

		if k == "PG_DEGRADED" {
			matched := degradedObjectsRegex.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 3 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				c.DegradedObjectsCount.Set(float64(v))
			}
		}

		if k == "OBJECT_MISPLACED" {
			matched := misplacedObjectsRegex.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 3 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				c.MisplacedObjectsCount.Set(float64(v))
			}
		}

		if k == "OSDMAP_FLAGS" {
			matched := osdmapFlagsRegex.FindStringSubmatch(check.Summary.Message)
			if len(matched) > 0 {
				flags := strings.Split(matched[1], ",")
				for _, f := range flags {
					switch f {
					case "full":
						c.OSDMapFlagFull.Set(1)
					case "pauserd":
						c.OSDMapFlagPauseRd.Set(1)
					case "pausewr":
						c.OSDMapFlagPauseWr.Set(1)
					case "noup":
						c.OSDMapFlagNoUp.Set(1)
					case "nodown":
						c.OSDMapFlagNoDown.Set(1)
					case "noin":
						c.OSDMapFlagNoIn.Set(1)
					case "noout":
						c.OSDMapFlagNoOut.Set(1)
					case "nobackfill":
						c.OSDMapFlagNoBackfill.Set(1)
					case "norecover":
						c.OSDMapFlagNoRecover.Set(1)
					case "norebalance":
						c.OSDMapFlagNoRebalance.Set(1)
					case "noscrub":
						c.OSDMapFlagNoScrub.Set(1)
					case "nodeep_scrub":
						c.OSDMapFlagNoDeepScrub.Set(1)
					case "notieragent":
						c.OSDMapFlagNoTierAgent.Set(1)
					}
				}
			}
		}
	}

	var (
		degradedPGs       float64
		activePGs         float64
		uncleanPGs        float64
		undersizedPGs     float64
		peeringPGs        float64
		stalePGs          float64
		scrubbingPGs      float64
		deepScrubbingPGs  float64
		recoveringPGs     float64
		recoveryWaitPGs   float64
		backfillingPGs    float64
		backfillWaitPGs   float64
		forcedRecoveryPGs float64
		forcedBackfillPGs float64
		downPGs           float64

		pgStateMap = map[string]*float64{
			"degraded":        &degradedPGs,
			"active":          &activePGs,
			"unclean":         &uncleanPGs,
			"undersized":      &undersizedPGs,
			"peering":         &peeringPGs,
			"stale":           &stalePGs,
			"scrubbing":       &scrubbingPGs,
			"scrubbing+deep":  &deepScrubbingPGs,
			"recovering":      &recoveringPGs,
			"recovery_wait":   &recoveryWaitPGs,
			"backfilling":     &backfillingPGs,
			"backfill_wait":   &backfillWaitPGs,
			"forced_recovery": &forcedRecoveryPGs,
			"forced_backfill": &forcedBackfillPGs,
			"down":            &downPGs,
		}
	)

	for _, p := range stats.PGMap.PGsByState {
		for pgState := range pgStateMap {
			if strings.Contains(p.States, pgState) {
				*pgStateMap[pgState] += p.Count
			}
		}
	}

	if *pgStateMap["degraded"] > 0 {
		c.DegradedPGs.Set(*pgStateMap["degraded"])
	}
	if *pgStateMap["active"] > 0 {
		c.ActivePGs.Set(*pgStateMap["active"])
	}
	if *pgStateMap["unclean"] > 0 {
		c.UncleanPGs.Set(*pgStateMap["unclean"])
	}
	if *pgStateMap["undersized"] > 0 {
		c.UndersizedPGs.Set(*pgStateMap["undersized"])
	}
	if *pgStateMap["peering"] > 0 {
		c.PeeringPGs.Set(*pgStateMap["peering"])
	}
	if *pgStateMap["stale"] > 0 {
		c.StalePGs.Set(*pgStateMap["stale"])
	}
	if *pgStateMap["scrubbing"] > 0 {
		c.ScrubbingPGs.Set(*pgStateMap["scrubbing"] - *pgStateMap["scrubbing+deep"])
	}
	if *pgStateMap["scrubbing+deep"] > 0 {
		c.DeepScrubbingPGs.Set(*pgStateMap["scrubbing+deep"])
	}
	if *pgStateMap["recovering"] > 0 {
		c.RecoveringPGs.Set(*pgStateMap["recovering"])
	}
	if *pgStateMap["recovery_wait"] > 0 {
		c.RecoveryWaitPGs.Set(*pgStateMap["recovery_wait"])
	}
	if *pgStateMap["backfilling"] > 0 {
		c.BackfillingPGs.Set(*pgStateMap["backfilling"])
	}
	if *pgStateMap["backfill_wait"] > 0 {
		c.BackfillWaitPGs.Set(*pgStateMap["backfill_wait"])
	}
	if *pgStateMap["forced_recovery"] > 0 {
		c.ForcedRecoveryPGs.Set(*pgStateMap["forced_recovery"])
	}
	if *pgStateMap["forced_backfill"] > 0 {
		c.ForcedBackfillPGs.Set(*pgStateMap["forced_backfill"])
	}
	if *pgStateMap["down"] > 0 {
		c.DownPGs.Set(*pgStateMap["down"])
	}

	c.ClientReadBytesPerSec.Set(stats.PGMap.ReadBytePerSec)
	c.ClientWriteBytesPerSec.Set(stats.PGMap.WriteBytePerSec)
	c.ClientIOOps.Set(stats.PGMap.ReadOpPerSec + stats.PGMap.WriteOpPerSec)
	c.ClientIOReadOps.Set(stats.PGMap.ReadOpPerSec)
	c.ClientIOWriteOps.Set(stats.PGMap.WriteOpPerSec)
	c.RecoveryIOKeys.Set(stats.PGMap.RecoveringKeysPerSec)
	c.RecoveryIOObjects.Set(stats.PGMap.RecoveringObjectsPerSec)
	c.RecoveryIORate.Set(stats.PGMap.RecoveringBytePerSec)
	c.CacheEvictIORate.Set(stats.PGMap.CacheEvictBytePerSec)
	c.CacheFlushIORate.Set(stats.PGMap.CacheFlushBytePerSec)
	c.CachePromoteIOOps.Set(stats.PGMap.CachePromoteOpPerSec)

	c.OSDsUp.Set(stats.OSDMap.OSDMap.NumUpOSDs)
	c.OSDsIn.Set(stats.OSDMap.OSDMap.NumInOSDs)
	c.OSDsNum.Set(stats.OSDMap.OSDMap.NumOSDs)

	// Ceph (until v10.2.3) doesn't expose the value of down OSDs
	// from its status, which is why we have to compute it ourselves.
	c.OSDsDown.Set(stats.OSDMap.OSDMap.NumOSDs - stats.OSDMap.OSDMap.NumUpOSDs)

	c.RemappedPGs.Set(stats.OSDMap.OSDMap.NumRemappedPGs)
	c.TotalPGs.Set(stats.PGMap.NumPGs)

	for _, checkType := range []string{"REQUEST_SLOW", "REQUEST_STUCK"} {
		if err := c.calculateSlowRequestsPerOSD(ch, checkType); err != nil {
			return err
		}
	}

	return nil
}

func (c *ClusterHealthCollector) calculateSlowRequestsPerOSD(ch chan<- prometheus.Metric, checkType string) error {
	var (
		slowOpsBlockedRegex         = regexp.MustCompile(`([\d]+) ops are blocked > ([\d\.]+) sec`)
		slowRequestSingleOSDRegex   = regexp.MustCompile(`osd.([\d]+) has blocked requests > ([\d\.]+) sec`)
		slowRequestMultipleOSDRegex = regexp.MustCompile(`osds ([\d,]+) have blocked requests > ([\d\.]+) sec`)

		slowRequestStuckSingleOSDRegex   = regexp.MustCompile(`osd.([\d]+) has stuck requests > ([\d\.]+) sec`)
		slowRequestStuckMultipleOSDRegex = regexp.MustCompile(`osds ([\d,]+) have stuck requests > ([\d\.]+) sec`)

		secToOpsBlocked     = make(map[float64]int)
		osdToSecondsBlocked = make(map[int]float64)
	)

	cmd := c.cephHealthDetailCommand()
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return err
	}

	hdstats := &cephHealthDetailStats{}
	if err := json.Unmarshal(buf, hdstats); err != nil {
		return err
	}

	for key, check := range hdstats.Checks {
		if key == checkType {
			for _, detail := range check.Details {
				matched := slowOpsBlockedRegex.FindStringSubmatch(detail.Message)
				if len(matched) == 3 {
					v, err := strconv.Atoi(matched[1])
					if err != nil {
						return err
					}

					f, err := strconv.ParseFloat(matched[2], 64)
					if err != nil {
						return err
					}

					secToOpsBlocked[f] = v
					continue
				}

				matched = slowRequestSingleOSDRegex.FindStringSubmatch(detail.Message)
				if len(matched) == 3 {
					v, err := strconv.Atoi(matched[1])
					if err != nil {
						return err
					}

					f, err := strconv.ParseFloat(matched[2], 64)
					if err != nil {
						return err
					}

					osdToSecondsBlocked[v] = f
					continue
				}

				matched = slowRequestMultipleOSDRegex.FindStringSubmatch(detail.Message)
				if len(matched) == 3 {
					f, err := strconv.ParseFloat(matched[2], 64)
					if err != nil {
						return err
					}

					for _, osdID := range strings.Split(matched[1], ",") {
						oid, err := strconv.Atoi(osdID)
						if err != nil {
							return err
						}

						osdToSecondsBlocked[oid] = f
					}
					continue
				}

				matched = slowRequestStuckSingleOSDRegex.FindStringSubmatch(detail.Message)
				if len(matched) == 3 {
					v, err := strconv.Atoi(matched[1])
					if err != nil {
						return err
					}

					f, err := strconv.ParseFloat(matched[2], 64)
					if err != nil {
						return err
					}

					osdToSecondsBlocked[v] = f
					continue
				}

				matched = slowRequestStuckMultipleOSDRegex.FindStringSubmatch(detail.Message)
				if len(matched) == 3 {
					f, err := strconv.ParseFloat(matched[2], 64)
					if err != nil {
						return err
					}

					for _, osdID := range strings.Split(matched[1], ",") {
						oid, err := strconv.Atoi(osdID)
						if err != nil {
							return err
						}

						osdToSecondsBlocked[oid] = f
					}
					continue
				}
			}
		}
	}

	secs := make([]float64, len(secToOpsBlocked))
	for sec := range secToOpsBlocked {
		secs = append(secs, sec)
	}
	sort.Float64s(secs)

	totalOpsUntilNow := 0
	totalOpsSet := false
	for _, sec := range secs {
		totalOpsUntilNow += secToOpsBlocked[sec]
		for osd, osec := range osdToSecondsBlocked {
			if sec == osec {
				ch <- prometheus.MustNewConstMetric(c.SlowRequestsByOSDDesc, prometheus.GaugeValue, float64(totalOpsUntilNow), strconv.Itoa(osd))
				totalOpsSet = true
			}
		}

		if totalOpsSet {
			totalOpsUntilNow = 0
			totalOpsSet = false
		}
	}

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

func (c *ClusterHealthCollector) cephHealthDetailCommand() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "health",
		"detail": "detail",
		"format": jsonFormat,
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

		// If we discover the health check is Luminous-specific
		// we stop continuing extracting recovery/client I/O,
		// because we already get it from health function.
		if line == "cluster:" {
			return nil
		}

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
	matched := clientReadBytesPerSecRegex.FindStringSubmatch(clientStr)
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

		c.ClientReadBytesPerSec.Set(float64(v))
	}

	matched = clientWriteBytesPerSecRegex.FindStringSubmatch(clientStr)
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

		c.ClientWriteBytesPerSec.Set(float64(v))
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

	var ClientIOReadOps, ClientIOWriteOps float64
	matched = clientIOReadOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		ClientIOReadOps = float64(v)
		c.ClientIOReadOps.Set(ClientIOReadOps)
	}

	matched = clientIOWriteOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		ClientIOWriteOps = float64(v)
		c.ClientIOWriteOps.Set(ClientIOWriteOps)
	}

	// In versions older than Jewel, we directly get access to total
	// client I/O. But in Jewel and newer the format is changed to
	// separately display read and write IOPs. In such a case, we
	// compute and set the total IOPs ourselves.
	if clientIOOps == 0 {
		clientIOOps = ClientIOReadOps + ClientIOWriteOps
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
	ch <- c.SlowRequestsByOSDDesc

	for _, metric := range c.metricsList() {
		ch <- metric.Desc()
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (c *ClusterHealthCollector) Collect(ch chan<- prometheus.Metric) {
	if err := c.collect(ch); err != nil {
		log.Println("failed collecting cluster health metrics:", err)
	}

	if err := c.collectRecoveryClientIO(); err != nil {
		log.Println("failed collecting cluster recovery/client io:", err)
	}

	for _, metric := range c.metricsList() {
		ch <- metric
	}
}
