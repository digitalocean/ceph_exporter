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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
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
	conn    Conn
	logger  *logrus.Logger
	version *Version

	// healthChecksMap stores warnings and their criticality
	healthChecksMap map[string]int

	// HealthStatus shows the overall health status of a given cluster.
	HealthStatus *prometheus.Desc

	// HealthStatusInterpreter shows the overall health status of a given
	// cluster, with a breakdown of the HEALTH_WARN status into two groups
	// based on criticality.
	HealthStatusInterpreter prometheus.Gauge

	// MONsDown show the no. of Monitor that are int DOWN state
	MONsDown *prometheus.Desc

	// TotalPGs shows the total no. of PGs the cluster constitutes of.
	TotalPGs *prometheus.Desc

	// PGstate contains state of all PGs labelled with the name of states.
	PGState *prometheus.Desc

	// ActivePGs shows the no. of PGs the cluster is actively serving data
	// from.
	ActivePGs *prometheus.Desc

	// DegradedPGs shows the no. of PGs that have some of the replicas
	// missing.
	DegradedPGs *prometheus.Desc

	// StuckDegradedPGs shows the no. of PGs that have some of the replicas
	// missing, and are stuck in that state.
	StuckDegradedPGs *prometheus.Desc

	// UncleanPGs shows the no. of PGs that do not have all objects in the PG
	// that are supposed to be in it.
	UncleanPGs *prometheus.Desc

	// StuckUncleanPGs shows the no. of PGs that do not have all objects in the PG
	// that are supposed to be in it, and are stuck in that state.
	StuckUncleanPGs *prometheus.Desc

	// UndersizedPGs depicts the no. of PGs that have fewer copies than configured
	// replication level.
	UndersizedPGs *prometheus.Desc

	// StuckUndersizedPGs depicts the no. of PGs that have fewer copies than configured
	// replication level, and are stuck in that state.
	StuckUndersizedPGs *prometheus.Desc

	// StalePGs depicts no. of PGs that are in an unknown state i.e. monitors do not know
	// anything about their latest state since their pg mapping was modified.
	StalePGs *prometheus.Desc

	// StuckStalePGs depicts no. of PGs that are in an unknown state i.e. monitors do not know
	// anything about their latest state since their pg mapping was modified, and are stuck
	// in that state.
	StuckStalePGs *prometheus.Desc

	// PeeringPGs depicts no. of PGs that have one or more OSDs undergo state changes
	// that need to be communicated to the remaining peers.
	PeeringPGs *prometheus.Desc

	// ScrubbingPGs depicts no. of PGs that are in scrubbing state.
	// Light scrubbing checks the object size and attributes.
	ScrubbingPGs *prometheus.Desc

	// DeepScrubbingPGs depicts no. of PGs that are in scrubbing+deep state.
	// Deep scrubbing reads the data and uses checksums to ensure data integrity.
	DeepScrubbingPGs *prometheus.Desc

	// RecoveringPGs depicts no. of PGs that are in recovering state.
	// The PGs in this state have been dequeued from recovery_wait queue and are
	// actively undergoing recovery.
	RecoveringPGs *prometheus.Desc

	// RecoveryWaitPGs depicts no. of PGs that are in recovery_wait state.
	// The PGs in this state are still in queue to start recovery on them.
	RecoveryWaitPGs *prometheus.Desc

	// BackfillingPGs depicts no. of PGs that are in backfilling state.
	// The PGs in this state have been dequeued from backfill_wait queue and are
	// actively undergoing recovery.
	BackfillingPGs *prometheus.Desc

	// BackfillWaitPGs depicts no. of PGs that are in backfill_wait state.
	// The PGs in this state are still in queue to start backfill on them.
	BackfillWaitPGs *prometheus.Desc

	// ForcedRecoveryPGs depicts no. of PGs that are undergoing forced recovery.
	ForcedRecoveryPGs *prometheus.Desc

	// ForcedBackfillPGs depicts no. of PGs that are undergoing forced backfill.
	ForcedBackfillPGs *prometheus.Desc

	// DownPGs depicts no. of PGs that are currently down and not able to serve traffic.
	DownPGs *prometheus.Desc

	// IncompletePGs depicts no. of PGs that are currently incomplete and not able to serve traffic.
	IncompletePGs *prometheus.Desc

	// InconsistentPGs depicts no. of PGs that are currently inconsistent
	InconsistentPGs *prometheus.Desc

	// SnaptrimPGs depicts no. of PGs that are currently snaptrimming
	SnaptrimPGs *prometheus.Desc

	// SnaptrimWaitPGs depicts no. of PGs that are currently waiting to snaptrim
	SnaptrimWaitPGs *prometheus.Desc

	// RepairingPGs depicts no. of PGs that are currently repairing
	RepairingPGs *prometheus.Desc

	// SlowOps depicts no. of total slow ops in the cluster
	SlowOps *prometheus.Desc

	// DegradedObjectsCount gives the no. of RADOS objects are constitute the degraded PGs.
	// This includes object replicas in its count.
	DegradedObjectsCount *prometheus.Desc

	// MisplacedObjectsCount gives the no. of RADOS objects that constitute the misplaced PGs.
	// Misplaced PGs usually represent the PGs that are not in the storage locations that
	// they should be in. This is different than degraded PGs which means a PG has fewer copies
	// that it should.
	// This includes object replicas in its count.
	MisplacedObjectsCount *prometheus.Desc

	// MisplacedRatio shows the ratio of misplaced objects to total objects
	MisplacedRatio *prometheus.Desc

	// NewCrashReportCount reports if new Ceph daemon crash reports are available
	NewCrashReportCount *prometheus.Desc

	// TooManyRepairs reports the number of OSDs exceeding mon_osd_warn_num_repaired
	TooManyRepairs *prometheus.Desc

	// Objects show the total no. of RADOS objects that are currently allocated
	Objects *prometheus.Desc

	// OSDMapFlags - **these are being deprecated in favor of using the OSDMapFlags ConstMetrics descriptor**
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

	// OSDMapFlags, but implemented as a ConstMetric and each flag is a label
	OSDMapFlags *prometheus.Desc
	// OSDFlagToGaugeMap maps flags to gauges
	OSDFlagToGaugeMap map[string]*prometheus.Gauge

	// OSDsDown show the no. of OSDs that are in the DOWN state.
	OSDsDown *prometheus.Desc

	// OSDsUp show the no. of OSDs that are in the UP state and are able to serve requests.
	OSDsUp *prometheus.Desc

	// OSDsIn shows the no. of OSDs that are marked as IN in the cluster.
	OSDsIn *prometheus.Desc

	// OSDsNum shows the no. of total OSDs the cluster has.
	OSDsNum *prometheus.Desc

	// RemappedPGs show the no. of PGs that are currently remapped and needs to be moved
	// to newer OSDs.
	RemappedPGs *prometheus.Desc

	// RecoveryIORate shows the i/o rate at which the cluster is performing its ongoing
	// recovery at.
	RecoveryIORate *prometheus.Desc

	// RecoveryIOKeys shows the rate of rados keys recovery.
	RecoveryIOKeys *prometheus.Desc

	// RecoveryIOObjects shows the rate of rados objects being recovered.
	RecoveryIOObjects *prometheus.Desc

	// ClientReadBytesPerSec shows the total client read i/o on the cluster.
	ClientReadBytesPerSec *prometheus.Desc

	// ClientWriteBytesPerSec shows the total client write i/o on the cluster.
	ClientWriteBytesPerSec *prometheus.Desc

	// ClientIOOps shows the rate of total operations conducted by all clients on the cluster.
	ClientIOOps *prometheus.Desc

	// ClientIOReadOps shows the rate of total read operations conducted by all clients on the cluster.
	ClientIOReadOps *prometheus.Desc

	// ClientIOWriteOps shows the rate of total write operations conducted by all clients on the cluster.
	ClientIOWriteOps *prometheus.Desc

	// CacheFlushIORate shows the i/o rate at which data is being flushed from the cache pool.
	CacheFlushIORate *prometheus.Desc

	// CacheEvictIORate shows the i/o rate at which data is being flushed from the cache pool.
	CacheEvictIORate *prometheus.Desc

	// CachePromoteIOOps shows the rate of operations promoting objects to the cache pool.
	CachePromoteIOOps *prometheus.Desc

	// MgrsActive shows the number of active mgrs, can be either 0 or 1.
	MgrsActive *prometheus.Desc

	// MgrsNum shows the total number of mgrs, including standbys.
	MgrsNum *prometheus.Desc

	// RbdMirrorUp shows the alive rbd-mirror daemons
	RbdMirrorUp *prometheus.Desc
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
func NewClusterHealthCollector(exporter *Exporter) *ClusterHealthCollector {
	labels := make(prometheus.Labels)
	labels["cluster"] = exporter.Cluster

	collector := &ClusterHealthCollector{
		conn:    exporter.Conn,
		logger:  exporter.Logger,
		version: exporter.Version,

		healthChecksMap: map[string]int{
			"AUTH_BAD_CAPS":                        2,
			"BLUEFS_AVAILABLE_SPACE":               1,
			"BLUEFS_LOW_SPACE":                     1,
			"BLUEFS_SPILLOVER":                     1,
			"BLUESTORE_DISK_SIZE_MISMATCH":         1,
			"BLUESTORE_FRAGMENTATION":              1,
			"BLUESTORE_LEGACY_STATFS":              1,
			"BLUESTORE_NO_COMPRESSION":             1,
			"BLUESTORE_NO_PER_POOL_MAP":            1,
			"CACHE_POOL_NEAR_FULL":                 1,
			"CACHE_POOL_NO_HIT_SET":                1,
			"DEVICE_HEALTH":                        1,
			"DEVICE_HEALTH_IN_USE":                 2,
			"DEVICE_HEALTH_TOOMANY":                2,
			"LARGE_OMAP_OBJECTS":                   1,
			"MANY_OBJECTS_PER_PG":                  1,
			"MGR_DOWN":                             2,
			"MGR_MODULE_DEPENDENCY":                1,
			"MGR_MODULE_ERROR":                     2,
			"MON_CLOCK_SKEW":                       2,
			"MON_DISK_BIG":                         1,
			"MON_DISK_CRIT":                        2,
			"MON_DISK_LOW":                         2,
			"MON_DOWN":                             2,
			"MON_MSGR2_NOT_ENABLED":                2,
			"OBJECT_MISPLACED":                     1,
			"OBJECT_UNFOUND":                       2,
			"OLD_CRUSH_STRAW_CALC_VERSION":         1,
			"OLD_CRUSH_TUNABLES":                   2,
			"OSDMAP_FLAGS":                         1,
			"OSD_BACKFILLFULL":                     2,
			"OSD_CHASSIS_DOWN":                     1,
			"OSD_DATACENTER_DOWN":                  1,
			"OSD_DOWN":                             1,
			"OSD_FLAGS":                            1,
			"OSD_FULL":                             2,
			"OSD_HOST_DOWN":                        1,
			"OSD_NEARFULL":                         2,
			"OSD_NO_DOWN_OUT_INTERVAL":             2,
			"OSD_NO_SORTBITWISE":                   2,
			"OSD_ORPHAN":                           2,
			"OSD_OSD_DOWN":                         1,
			"OSD_OUT_OF_ORDER_FULL":                2,
			"OSD_PDU_DOWN":                         1,
			"OSD_POD_DOWN":                         1,
			"OSD_RACK_DOWN":                        1,
			"OSD_REGION_DOWN":                      1,
			"OSD_ROOM_DOWN":                        1,
			"OSD_ROOT_DOWN":                        1,
			"OSD_ROW_DOWN":                         1,
			"OSD_SCRUB_ERRORS":                     2,
			"OSD_TOO_MANY_REPAIRS":                 1,
			"PG_AVAILABILITY":                      1,
			"PG_BACKFILL_FULL":                     2,
			"PG_DAMAGED":                           2,
			"PG_DEGRADED":                          1,
			"PG_NOT_DEEP_SCRUBBED":                 1,
			"PG_NOT_SCRUBBED":                      1,
			"PG_RECOVERY_FULL":                     2,
			"PG_SLOW_SNAP_TRIMMING":                1,
			"POOL_APP_NOT_ENABLED":                 2,
			"POOL_FULL":                            2,
			"POOL_NEAR_FULL":                       2,
			"POOL_TARGET_SIZE_BYTES_OVERCOMMITTED": 1,
			"POOL_TARGET_SIZE_RATIO_OVERCOMMITTED": 1,
			"POOL_TOO_FEW_PGS":                     1,
			"POOL_TOO_MANY_PGS":                    1,
			"RECENT_CRASH":                         1,
			"SLOW_OPS":                             1,
			"SMALLER_PGP_NUM":                      1,
			"TELEMETRY_CHANGED":                    1,
			"TOO_FEW_OSDS":                         1,
			"TOO_FEW_PGS":                          1,
			"TOO_MANY_PGS":                         1},

		HealthStatus: prometheus.NewDesc(fmt.Sprintf("%s_health_status", cephNamespace), "Health status of Cluster, can vary only between 3 states (err:2, warn:1, ok:0)", nil, labels),
		//HealthStatusInterpreter: prometheus.NewDesc(fmt.Sprintf("%s_health_status_interp", cephNamespace), "Health status of Cluster, can vary only between 4 states (err:3, critical_warn:2, soft_warn:1, ok:0)", nil, labels),
		HealthStatusInterpreter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace:   cephNamespace,
				Name:        "health_status_interp",
				Help:        "Health status of Cluster, can vary only between 4 states (err:3, critical_warn:2, soft_warn:1, ok:0)",
				ConstLabels: labels,
			},
		),
		MONsDown:          prometheus.NewDesc(fmt.Sprintf("%s_mons_down", cephNamespace), "Count of Mons that are in DOWN state", nil, labels),
		TotalPGs:          prometheus.NewDesc(fmt.Sprintf("%s_total_pgs", cephNamespace), "Total no. of PGs in the cluster", nil, labels),
		PGState:           prometheus.NewDesc(fmt.Sprintf("%s_pg_state", cephNamespace), "State of PGs in the cluster", []string{"state"}, labels),
		ActivePGs:         prometheus.NewDesc(fmt.Sprintf("%s_active_pgs", cephNamespace), "No. of active PGs in the cluster", nil, labels),
		ScrubbingPGs:      prometheus.NewDesc(fmt.Sprintf("%s_scrubbing_pgs", cephNamespace), "No. of scrubbing PGs in the cluster", nil, labels),
		DeepScrubbingPGs:  prometheus.NewDesc(fmt.Sprintf("%s_deep_scrubbing_pgs", cephNamespace), "No. of deep scrubbing PGs in the cluster", nil, labels),
		RecoveringPGs:     prometheus.NewDesc(fmt.Sprintf("%s_recovering_pgs", cephNamespace), "No. of recovering PGs in the cluster", nil, labels),
		RecoveryWaitPGs:   prometheus.NewDesc(fmt.Sprintf("%s_recovery_wait_pgs", cephNamespace), "No. of PGs in the cluster with recovery_wait state", nil, labels),
		BackfillingPGs:    prometheus.NewDesc(fmt.Sprintf("%s_backfilling_pgs", cephNamespace), "No. of backfilling PGs in the cluster", nil, labels),
		BackfillWaitPGs:   prometheus.NewDesc(fmt.Sprintf("%s_backfill_wait_pgs", cephNamespace), "No. of PGs in the cluster with backfill_wait state", nil, labels),
		ForcedRecoveryPGs: prometheus.NewDesc(fmt.Sprintf("%s_forced_recovery_pgs", cephNamespace), "No. of PGs in the cluster with forced_recovery state", nil, labels),
		ForcedBackfillPGs: prometheus.NewDesc(fmt.Sprintf("%s_forced_backfill_pgs", cephNamespace), "No. of PGs in the cluster with forced_backfill state", nil, labels),
		DownPGs:           prometheus.NewDesc(fmt.Sprintf("%s_down_pgs", cephNamespace), "No. of PGs in the cluster in down state", nil, labels),
		IncompletePGs:     prometheus.NewDesc(fmt.Sprintf("%s_incomplete_pgs", cephNamespace), "No. of PGs in the cluster in incomplete state", nil, labels),
		InconsistentPGs:   prometheus.NewDesc(fmt.Sprintf("%s_inconsistent_pgs", cephNamespace), "No. of PGs in the cluster in inconsistent state", nil, labels),
		SnaptrimPGs:       prometheus.NewDesc(fmt.Sprintf("%s_snaptrim_pgs", cephNamespace), "No. of snaptrim PGs in the cluster", nil, labels),
		SnaptrimWaitPGs:   prometheus.NewDesc(fmt.Sprintf("%s_snaptrim_wait_pgs", cephNamespace), "No. of PGs in the cluster with snaptrim_wait state", nil, labels),
		RepairingPGs:      prometheus.NewDesc(fmt.Sprintf("%s_repairing_pgs", cephNamespace), "No. of PGs in the cluster with repair state", nil, labels),
		// with Nautilus, SLOW_OPS has replaced both REQUEST_SLOW and REQUEST_STUCK
		// therefore slow_requests is deprecated, but for backwards compatibility
		// the metric name will be kept the same for the time being
		SlowOps:               prometheus.NewDesc(fmt.Sprintf("%s_slow_requests", cephNamespace), "No. of slow requests/slow ops", nil, labels),
		DegradedPGs:           prometheus.NewDesc(fmt.Sprintf("%s_degraded_pgs", cephNamespace), "No. of PGs in a degraded state", nil, labels),
		StuckDegradedPGs:      prometheus.NewDesc(fmt.Sprintf("%s_stuck_degraded_pgs", cephNamespace), "No. of PGs stuck in a degraded state", nil, labels),
		UncleanPGs:            prometheus.NewDesc(fmt.Sprintf("%s_unclean_pgs", cephNamespace), "No. of PGs in an unclean state", nil, labels),
		StuckUncleanPGs:       prometheus.NewDesc(fmt.Sprintf("%s_stuck_unclean_pgs", cephNamespace), "No. of PGs stuck in an unclean state", nil, labels),
		UndersizedPGs:         prometheus.NewDesc(fmt.Sprintf("%s_undersized_pgs", cephNamespace), "No. of undersized PGs in the cluster", nil, labels),
		StuckUndersizedPGs:    prometheus.NewDesc(fmt.Sprintf("%s_stuck_undersized_pgs", cephNamespace), "No. of stuck undersized PGs in the cluster", nil, labels),
		StalePGs:              prometheus.NewDesc(fmt.Sprintf("%s_stale_pgs", cephNamespace), "No. of stale PGs in the cluster", nil, labels),
		StuckStalePGs:         prometheus.NewDesc(fmt.Sprintf("%s_stuck_stale_pgs", cephNamespace), "No. of stuck stale PGs in the cluster", nil, labels),
		PeeringPGs:            prometheus.NewDesc(fmt.Sprintf("%s_peering_pgs", cephNamespace), "No. of peering PGs in the cluster", nil, labels),
		DegradedObjectsCount:  prometheus.NewDesc(fmt.Sprintf("%s_degraded_objects", cephNamespace), "No. of degraded objects across all PGs, includes replicas", nil, labels),
		MisplacedObjectsCount: prometheus.NewDesc(fmt.Sprintf("%s_misplaced_objects", cephNamespace), "No. of misplaced objects across all PGs, includes replicas", nil, labels),
		MisplacedRatio:        prometheus.NewDesc(fmt.Sprintf("%s_misplaced_ratio", cephNamespace), "ratio of misplaced objects to total objects", nil, labels),
		NewCrashReportCount:   prometheus.NewDesc(fmt.Sprintf("%s_new_crash_reports", cephNamespace), "Number of new crash reports available", nil, labels),
		TooManyRepairs:        prometheus.NewDesc(fmt.Sprintf("%s_osds_too_many_repair", cephNamespace), "Number of OSDs with too many repaired reads", nil, labels),
		Objects:               prometheus.NewDesc(fmt.Sprintf("%s_cluster_objects", cephNamespace), "No. of rados objects within the cluster", nil, labels),
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

		OSDMapFlags:            prometheus.NewDesc(fmt.Sprintf("%s_osd_map_flags", cephNamespace), "A metric for all OSDMap flags", []string{"flag"}, labels),
		OSDsDown:               prometheus.NewDesc(fmt.Sprintf("%s_osds_down", cephNamespace), "Count of OSDs that are in DOWN state", nil, labels),
		OSDsUp:                 prometheus.NewDesc(fmt.Sprintf("%s_osds_up", cephNamespace), "Count of OSDs that are in UP state", nil, labels),
		OSDsIn:                 prometheus.NewDesc(fmt.Sprintf("%s_osds_in", cephNamespace), "Count of OSDs that are in IN state and available to serve requests", nil, labels),
		OSDsNum:                prometheus.NewDesc(fmt.Sprintf("%s_osds", cephNamespace), "Count of total OSDs in the cluster", nil, labels),
		RemappedPGs:            prometheus.NewDesc(fmt.Sprintf("%s_pgs_remapped", cephNamespace), "No. of PGs that are remapped and incurring cluster-wide movement", nil, labels),
		RecoveryIORate:         prometheus.NewDesc(fmt.Sprintf("%s_recovery_io_bytes", cephNamespace), "Rate of bytes being recovered in cluster per second", nil, labels),
		RecoveryIOKeys:         prometheus.NewDesc(fmt.Sprintf("%s_recovery_io_keys", cephNamespace), "Rate of keys being recovered in cluster per second", nil, labels),
		RecoveryIOObjects:      prometheus.NewDesc(fmt.Sprintf("%s_recovery_io_objects", cephNamespace), "Rate of objects being recovered in cluster per second", nil, labels),
		ClientReadBytesPerSec:  prometheus.NewDesc(fmt.Sprintf("%s_client_io_read_bytes", cephNamespace), "Rate of bytes being read by all clients per second", nil, labels),
		ClientWriteBytesPerSec: prometheus.NewDesc(fmt.Sprintf("%s_client_io_write_bytes", cephNamespace), "Rate of bytes being written by all clients per second", nil, labels),
		ClientIOOps:            prometheus.NewDesc(fmt.Sprintf("%s_client_io_ops", cephNamespace), "Total client ops on the cluster measured per second", nil, labels),
		ClientIOReadOps:        prometheus.NewDesc(fmt.Sprintf("%s_client_io_read_ops", cephNamespace), "Total client read I/O ops on the cluster measured per second", nil, labels),
		ClientIOWriteOps:       prometheus.NewDesc(fmt.Sprintf("%s_client_io_write_ops", cephNamespace), "Total client write I/O ops on the cluster measured per second", nil, labels),
		CacheFlushIORate:       prometheus.NewDesc(fmt.Sprintf("%s_cache_flush_io_bytes", cephNamespace), "Rate of bytes being flushed from the cache pool per second", nil, labels),
		CacheEvictIORate:       prometheus.NewDesc(fmt.Sprintf("%s_cache_evict_io_bytes", cephNamespace), "Rate of bytes being evicted from the cache pool per second", nil, labels),
		CachePromoteIOOps:      prometheus.NewDesc(fmt.Sprintf("%s_cache_promote_io_ops", cephNamespace), "Total cache promote operations measured per second", nil, labels),
		MgrsActive:             prometheus.NewDesc(fmt.Sprintf("%s_mgrs_active", cephNamespace), "Count of active mgrs, can be either 0 or 1", nil, labels),
		MgrsNum:                prometheus.NewDesc(fmt.Sprintf("%s_mgrs", cephNamespace), "Total number of mgrs, including standbys", nil, labels),
		RbdMirrorUp:            prometheus.NewDesc(fmt.Sprintf("%s_rbd_mirror_up", cephNamespace), "Alive rbd-mirror daemons", []string{"name"}, labels),
	}

	// This is here to support backwards compatibility with gauges, but also exists as a general list of possible flags
	collector.OSDFlagToGaugeMap = map[string]*prometheus.Gauge{
		"full":         &collector.OSDMapFlagFull,
		"pauserd":      &collector.OSDMapFlagPauseRd,
		"pausewr":      &collector.OSDMapFlagPauseWr,
		"noup":         &collector.OSDMapFlagNoUp,
		"nodown":       &collector.OSDMapFlagNoDown,
		"noin":         &collector.OSDMapFlagNoIn,
		"noout":        &collector.OSDMapFlagNoOut,
		"nobackfill":   &collector.OSDMapFlagNoBackfill,
		"norecover":    &collector.OSDMapFlagNoRecover,
		"norebalance":  &collector.OSDMapFlagNoRebalance,
		"noscrub":      &collector.OSDMapFlagNoScrub,
		"nodeep_scrub": &collector.OSDMapFlagNoDeepScrub,
		"notieragent":  &collector.OSDMapFlagNoTierAgent,
	}

	if exporter.Version.IsAtLeast(Pacific) {
		// pacific adds the DAEMON_OLD_VERSION health check
		// that indicates that multiple versions of Ceph have been running for longer than mon_warn_older_version_delay
		// we'll interpret this is a critical warning (2)
		collector.healthChecksMap["DAEMON_OLD_VERSION"] = 2
	}

	return collector
}

// collectorsList represents legacy gauges before the migration to constmetrics
func (c *ClusterHealthCollector) collectorsList() []prometheus.Collector {
	return []prometheus.Collector{
		c.HealthStatusInterpreter,

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
	}
}

func (c *ClusterHealthCollector) descriptorList() []*prometheus.Desc {
	return []*prometheus.Desc{
		c.HealthStatus,
		c.HealthStatusInterpreter.Desc(),
		c.MONsDown,
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
		c.IncompletePGs,
		c.InconsistentPGs,
		c.SnaptrimPGs,
		c.SnaptrimWaitPGs,
		c.RepairingPGs,
		c.SlowOps,
		c.DegradedObjectsCount,
		c.MisplacedObjectsCount,
		c.MisplacedRatio,
		c.NewCrashReportCount,
		c.TooManyRepairs,
		c.Objects,
		c.OSDMapFlagFull.Desc(),
		c.OSDMapFlagPauseRd.Desc(),
		c.OSDMapFlagPauseWr.Desc(),
		c.OSDMapFlagNoUp.Desc(),
		c.OSDMapFlagNoDown.Desc(),
		c.OSDMapFlagNoIn.Desc(),
		c.OSDMapFlagNoOut.Desc(),
		c.OSDMapFlagNoBackfill.Desc(),
		c.OSDMapFlagNoRecover.Desc(),
		c.OSDMapFlagNoRebalance.Desc(),
		c.OSDMapFlagNoScrub.Desc(),
		c.OSDMapFlagNoDeepScrub.Desc(),
		c.OSDMapFlagNoTierAgent.Desc(),
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
		c.MgrsActive,
		c.MgrsNum,
		c.PGState,
	}
}

type osdMap struct {
	NumOSDs        float64 `json:"num_osds"`
	NumUpOSDs      float64 `json:"num_up_osds"`
	NumInOSDs      float64 `json:"num_in_osds"`
	NumRemappedPGs float64 `json:"num_remapped_pgs"`
}

type cephHealthStats struct {
	Health struct {
		Summary []struct {
			Severity string `json:"severity"`
			Summary  string `json:"summary"`
		} `json:"summary"`
		Status string `json:"status"`
		Checks map[string]struct {
			Severity string `json:"severity"`
			Summary  struct {
				Message string `json:"message"`
			} `json:"summary"`
		} `json:"checks"`
	} `json:"health"`
	OSDMap map[string]interface{} `json:"osdmap"`
	PGMap  struct {
		NumPGs                  float64 `json:"num_pgs"`
		TotalObjects            float64 `json:"num_objects"`
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
		MisplacedRatio          float64 `json:"misplaced_ratio"`
		PGsByState              []struct {
			Count  float64 `json:"count"`
			States string  `json:"state_name"`
		} `json:"pgs_by_state"`
	} `json:"pgmap"`
	MgrMap struct {
		// Octopus+ fields
		Available   bool `json:"available"`
		NumStandBys int  `json:"num_standbys"`

		// Nautilus fields
		ActiveName string `json:"active_name"`
		StandBys   []struct {
			Name string `json:"name"`
		} `json:"standbys"`
	} `json:"mgrmap"`
	ServiceMap struct {
		Services struct {
			RbdMirror struct {
				Daemons map[string]json.RawMessage `json:"daemons"`
			} `json:"rbd-mirror"`
		} `json:"services"`
	} `json:"servicemap"`
}

func (c *ClusterHealthCollector) collect(ch chan<- prometheus.Metric) error {
	cmd := c.cephUsageCommand(jsonFormat)
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		c.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

		return err
	}

	stats := &cephHealthStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return err
	}

	for _, metric := range c.collectorsList() {
		if gauge, ok := metric.(prometheus.Gauge); ok {
			gauge.Set(0)
		}
	}

	switch stats.Health.Status {
	case CephHealthOK:
		ch <- prometheus.MustNewConstMetric(c.HealthStatus, prometheus.GaugeValue, float64(0))
		c.HealthStatusInterpreter.Set(float64(0))
		// migration of HealthStatusInterpreter to ConstMetrics had to be reverted due to duplication issues with the current structure (and labels not being used)
		//ch <- prometheus.MustNewConstMetric(c.HealthStatusInterpreter, prometheus.GaugeValue, float64(0))
	case CephHealthWarn:
		ch <- prometheus.MustNewConstMetric(c.HealthStatus, prometheus.GaugeValue, float64(1))
		c.HealthStatusInterpreter.Set(float64(2))
		//ch <- prometheus.MustNewConstMetric(c.HealthStatusInterpreter, prometheus.GaugeValue, float64(2))
	case CephHealthErr:
		ch <- prometheus.MustNewConstMetric(c.HealthStatus, prometheus.GaugeValue, float64(2))
		c.HealthStatusInterpreter.Set(float64(3))
		//ch <- prometheus.MustNewConstMetric(c.HealthStatusInterpreter, prometheus.GaugeValue, float64(3))
	}

	var (
		monsDownRegex        = regexp.MustCompile(`([\d]+)/([\d]+) mons down, quorum \b+`)
		stuckDegradedRegex   = regexp.MustCompile(`([\d]+) pgs stuck degraded`)
		stuckUncleanRegex    = regexp.MustCompile(`([\d]+) pgs stuck unclean`)
		stuckUndersizedRegex = regexp.MustCompile(`([\d]+) pgs stuck undersized`)
		stuckStaleRegex      = regexp.MustCompile(`([\d]+) pgs stuck stale`)
		slowOpsRegexNautilus = regexp.MustCompile(`([\d]+) slow ops, oldest one blocked for ([\d]+) sec`)
		newCrashreportRegex  = regexp.MustCompile(`([\d]+) daemons have recently crashed`)
		tooManyRepairs       = regexp.MustCompile(`Too many repaired reads on ([\d]+) OSDs`)
		osdmapFlagsRegex     = regexp.MustCompile(`([^ ]+) flag\(s\) set`)
	)

	var mapEmpty = len(c.healthChecksMap) == 0

	for _, s := range stats.Health.Summary {
		matched := stuckDegradedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(c.StuckDegradedPGs, prometheus.GaugeValue, float64(v))
		}

		matched = stuckUncleanRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(c.StuckUncleanPGs, prometheus.GaugeValue, float64(v))
		}

		matched = stuckUndersizedRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(c.StuckUndersizedPGs, prometheus.GaugeValue, float64(v))
		}

		matched = stuckStaleRegex.FindStringSubmatch(s.Summary)
		if len(matched) == 2 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(c.StuckStalePGs, prometheus.GaugeValue, float64(v))
		}

		matched = slowOpsRegexNautilus.FindStringSubmatch(s.Summary)
		if len(matched) == 3 {
			v, err := strconv.Atoi(matched[1])
			if err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(c.SlowOps, prometheus.GaugeValue, float64(v))
		}
	}

	// This stores OSD map flags that were found, so the rest can be set to 0
	for k, check := range stats.Health.Checks {
		if k == "MON_DOWN" {
			matched := monsDownRegex.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 3 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				ch <- prometheus.MustNewConstMetric(c.MONsDown, prometheus.GaugeValue, float64(v))
			}
		}

		if k == "SLOW_OPS" {
			matched := slowOpsRegexNautilus.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 3 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				ch <- prometheus.MustNewConstMetric(c.SlowOps, prometheus.GaugeValue, float64(v))
			}
		}

		if k == "RECENT_CRASH" {
			matched := newCrashreportRegex.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 2 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				ch <- prometheus.MustNewConstMetric(c.NewCrashReportCount, prometheus.GaugeValue, float64(v))
			}
		}

		if k == "OSD_TOO_MANY_REPAIRS" {
			matched := tooManyRepairs.FindStringSubmatch(check.Summary.Message)
			if len(matched) == 2 {
				v, err := strconv.Atoi(matched[1])
				if err != nil {
					return err
				}
				ch <- prometheus.MustNewConstMetric(c.TooManyRepairs, prometheus.GaugeValue, float64(v))
			}
		}

		if k == "OSDMAP_FLAGS" {
			matched := osdmapFlagsRegex.FindStringSubmatch(check.Summary.Message)
			if len(matched) > 0 {
				flags := strings.Split(matched[1], ",")
				for _, f := range flags {
					// Update the global metric for this specific flag
					ch <- prometheus.MustNewConstMetric(c.OSDMapFlags, prometheus.GaugeValue, float64(1), f)
					// Update the legacy gauges, based on the map, if valid
					if _, exists := c.OSDFlagToGaugeMap[f]; exists {
						(*c.OSDFlagToGaugeMap[f]).Set(1)
					}
				}
			}
		}
		if !mapEmpty {
			if val, present := c.healthChecksMap[k]; present {
				c.HealthStatusInterpreter.Set(float64(val))
				// migration of HealthStatusInterpreter to ConstMetrics had to be reverted due to duplication issues with the current structure (and labels not being used)
				//ch <- prometheus.MustNewConstMetric(c.HealthStatusInterpreter, prometheus.GaugeValue, float64(val))
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
		incompletePGs     float64
		inconsistentPGs   float64
		snaptrimPGs       float64
		snaptrimWaitPGs   float64
		repairingPGs      float64

		pgStateCounterMap = map[string]*float64{
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
			"incomplete":      &incompletePGs,
			"inconsistent":    &inconsistentPGs,
			"snaptrim":        &snaptrimPGs,
			"snaptrim_wait":   &snaptrimWaitPGs,
			"repair":          &repairingPGs,
		}
		pgStateGaugeMap = map[string]*prometheus.Desc{
			"degraded":        c.DegradedPGs,
			"active":          c.ActivePGs,
			"unclean":         c.UncleanPGs,
			"undersized":      c.UndersizedPGs,
			"peering":         c.PeeringPGs,
			"stale":           c.StalePGs,
			"scrubbing":       c.ScrubbingPGs,
			"scrubbing+deep":  c.DeepScrubbingPGs,
			"recovering":      c.RecoveringPGs,
			"recovery_wait":   c.RecoveryWaitPGs,
			"backfilling":     c.BackfillingPGs,
			"backfill_wait":   c.BackfillWaitPGs,
			"forced_recovery": c.ForcedRecoveryPGs,
			"forced_backfill": c.ForcedBackfillPGs,
			"down":            c.DownPGs,
			"incomplete":      c.IncompletePGs,
			"inconsistent":    c.InconsistentPGs,
			"snaptrim":        c.SnaptrimPGs,
			"snaptrim_wait":   c.SnaptrimWaitPGs,
			"repair":          c.RepairingPGs,
		}
	)

	for _, p := range stats.PGMap.PGsByState {
		for pgState := range pgStateCounterMap {
			if strings.Contains(p.States, pgState) {
				*pgStateCounterMap[pgState] += p.Count
			}
		}
	}

	for state, gauge := range pgStateGaugeMap {
		val := *pgStateCounterMap[state]
		if state == "scrubbing" {
			val -= *pgStateCounterMap["scrubbing+deep"]
		}
		if state == "snaptrim" {
			val -= *pgStateCounterMap["snaptrim_wait"]
		}

		ch <- prometheus.MustNewConstMetric(gauge, prometheus.GaugeValue, val)

		if state == "scrubbing+deep" {
			state = "deep_scrubbing"
		}
		ch <- prometheus.MustNewConstMetric(c.PGState, prometheus.GaugeValue, val, state)
	}

	ch <- prometheus.MustNewConstMetric(c.ClientReadBytesPerSec, prometheus.GaugeValue, stats.PGMap.ReadBytePerSec)
	ch <- prometheus.MustNewConstMetric(c.ClientWriteBytesPerSec, prometheus.GaugeValue, stats.PGMap.WriteBytePerSec)
	ch <- prometheus.MustNewConstMetric(c.ClientIOOps, prometheus.GaugeValue, stats.PGMap.ReadOpPerSec+stats.PGMap.WriteOpPerSec)
	ch <- prometheus.MustNewConstMetric(c.ClientIOReadOps, prometheus.GaugeValue, stats.PGMap.ReadOpPerSec)
	ch <- prometheus.MustNewConstMetric(c.ClientIOWriteOps, prometheus.GaugeValue, stats.PGMap.WriteOpPerSec)
	ch <- prometheus.MustNewConstMetric(c.RecoveryIOKeys, prometheus.GaugeValue, stats.PGMap.RecoveringKeysPerSec)
	ch <- prometheus.MustNewConstMetric(c.RecoveryIOObjects, prometheus.GaugeValue, stats.PGMap.RecoveringObjectsPerSec)
	ch <- prometheus.MustNewConstMetric(c.RecoveryIORate, prometheus.GaugeValue, stats.PGMap.RecoveringBytePerSec)
	ch <- prometheus.MustNewConstMetric(c.CacheEvictIORate, prometheus.GaugeValue, stats.PGMap.CacheEvictBytePerSec)
	ch <- prometheus.MustNewConstMetric(c.CacheFlushIORate, prometheus.GaugeValue, stats.PGMap.CacheFlushBytePerSec)
	ch <- prometheus.MustNewConstMetric(c.CachePromoteIOOps, prometheus.GaugeValue, stats.PGMap.CachePromoteOpPerSec)

	var actualOsdMap osdMap
	if c.version.IsAtLeast(Octopus) {
		if stats.OSDMap != nil {
			actualOsdMap = osdMap{
				NumOSDs:        stats.OSDMap["num_osds"].(float64),
				NumUpOSDs:      stats.OSDMap["num_up_osds"].(float64),
				NumInOSDs:      stats.OSDMap["num_in_osds"].(float64),
				NumRemappedPGs: stats.OSDMap["num_remapped_pgs"].(float64),
			}
		}
	} else {
		if stats.OSDMap != nil {
			innerMap := stats.OSDMap["osdmap"].(map[string]interface{})

			actualOsdMap = osdMap{
				NumOSDs:        innerMap["num_osds"].(float64),
				NumUpOSDs:      innerMap["num_up_osds"].(float64),
				NumInOSDs:      innerMap["num_in_osds"].(float64),
				NumRemappedPGs: innerMap["num_remapped_pgs"].(float64),
			}
		}
	}

	ch <- prometheus.MustNewConstMetric(c.OSDsUp, prometheus.GaugeValue, actualOsdMap.NumUpOSDs)
	ch <- prometheus.MustNewConstMetric(c.OSDsIn, prometheus.GaugeValue, actualOsdMap.NumInOSDs)
	ch <- prometheus.MustNewConstMetric(c.OSDsNum, prometheus.GaugeValue, actualOsdMap.NumOSDs)

	// Ceph (until v10.2.3) doesn't expose the value of down OSDs
	// from its status, which is why we have to compute it ourselves.
	ch <- prometheus.MustNewConstMetric(c.OSDsDown, prometheus.GaugeValue, actualOsdMap.NumOSDs-actualOsdMap.NumUpOSDs)

	ch <- prometheus.MustNewConstMetric(c.RemappedPGs, prometheus.GaugeValue, actualOsdMap.NumRemappedPGs)
	ch <- prometheus.MustNewConstMetric(c.TotalPGs, prometheus.GaugeValue, stats.PGMap.NumPGs)
	ch <- prometheus.MustNewConstMetric(c.Objects, prometheus.GaugeValue, stats.PGMap.TotalObjects)

	ch <- prometheus.MustNewConstMetric(c.DegradedObjectsCount, prometheus.GaugeValue, stats.PGMap.DegradedObjects)
	ch <- prometheus.MustNewConstMetric(c.MisplacedObjectsCount, prometheus.GaugeValue, stats.PGMap.MisplacedObjects)
	ch <- prometheus.MustNewConstMetric(c.MisplacedRatio, prometheus.GaugeValue, stats.PGMap.MisplacedRatio)

	activeMgr := 0
	standByMgrs := 0
	if c.version.IsAtLeast(Octopus) {
		if stats.MgrMap.Available {
			activeMgr = 1
		}
		standByMgrs = stats.MgrMap.NumStandBys
	} else {
		if len(stats.MgrMap.ActiveName) > 0 {
			activeMgr = 1
		}
		standByMgrs = len(stats.MgrMap.StandBys)
	}

	ch <- prometheus.MustNewConstMetric(c.MgrsActive, prometheus.GaugeValue, float64(activeMgr))
	ch <- prometheus.MustNewConstMetric(c.MgrsNum, prometheus.GaugeValue, float64(activeMgr+standByMgrs))

	for name, data := range stats.ServiceMap.Services.RbdMirror.Daemons {
		if name == "summary" {
			continue
		}

		md := struct {
			Metadata struct {
				Id string `json:"id"`
			} `json:"metadata"`
		}{}

		// Extract id from metadata
		if err := json.Unmarshal(data, &md); err == nil {
			ch <- prometheus.MustNewConstMetric(
				c.RbdMirrorUp, prometheus.GaugeValue, 1.0, md.Metadata.Id)
		}
	}

	return nil
}

type format string

const (
	jsonFormat  format = "json"
	plainFormat format = "plain"
)

func (c *ClusterHealthCollector) cephUsageCommand(f format) []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "status",
		"format": f,
	})
	if err != nil {
		c.logger.WithError(err).Panic("error marshalling ceph status")
	}
	return cmd
}

func (c *ClusterHealthCollector) collectRecoveryClientIO(ch chan<- prometheus.Metric) error {
	cmd := c.cephUsageCommand(plainFormat)
	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		c.logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")

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
			if err := c.collectRecoveryIO(line, ch); err != nil {
				return err
			}
		case strings.HasPrefix(line, "recovery:"):
			if err := c.collectRecoveryIO(line, ch); err != nil {
				return err
			}
		case strings.HasPrefix(line, "client io"):
			if err := c.collectClientIO(line, ch); err != nil {
				return err
			}
		case strings.HasPrefix(line, "client:"):
			if err := c.collectClientIO(line, ch); err != nil {
				return err
			}
		case strings.HasPrefix(line, "cache io"):
			if err := c.collectCacheIO(line, ch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *ClusterHealthCollector) collectClientIO(clientStr string, ch chan<- prometheus.Metric) error {
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

		ch <- prometheus.MustNewConstMetric(c.ClientReadBytesPerSec, prometheus.GaugeValue, float64(v))
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

		ch <- prometheus.MustNewConstMetric(c.ClientWriteBytesPerSec, prometheus.GaugeValue, float64(v))
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
		ch <- prometheus.MustNewConstMetric(c.ClientIOReadOps, prometheus.GaugeValue, ClientIOReadOps)
	}

	matched = clientIOWriteOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		ClientIOWriteOps = float64(v)
		ch <- prometheus.MustNewConstMetric(c.ClientIOWriteOps, prometheus.GaugeValue, ClientIOWriteOps)
	}

	// In versions older than Jewel, we directly get access to total
	// client I/O. But in Jewel and newer the format is changed to
	// separately display read and write IOPs. In such a case, we
	// compute and set the total IOPs ourselves.
	if clientIOOps == 0 {
		clientIOOps = ClientIOReadOps + ClientIOWriteOps
	}

	ch <- prometheus.MustNewConstMetric(c.ClientIOOps, prometheus.GaugeValue, clientIOOps)

	return nil
}

func (c *ClusterHealthCollector) collectRecoveryIO(recoveryStr string, ch chan<- prometheus.Metric) error {
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

		ch <- prometheus.MustNewConstMetric(c.RecoveryIORate, prometheus.GaugeValue, float64(v))
	}

	matched = recoveryIOKeysRegex.FindStringSubmatch(recoveryStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(c.RecoveryIOKeys, prometheus.GaugeValue, float64(v))
	}

	matched = recoveryIOObjectsRegex.FindStringSubmatch(recoveryStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(c.RecoveryIOObjects, prometheus.GaugeValue, float64(v))
	}
	return nil
}

func (c *ClusterHealthCollector) collectCacheIO(clientStr string, ch chan<- prometheus.Metric) error {
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

		ch <- prometheus.MustNewConstMetric(c.CacheFlushIORate, prometheus.GaugeValue, float64(v))
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

		ch <- prometheus.MustNewConstMetric(c.CacheEvictIORate, prometheus.GaugeValue, float64(v))
	}

	matched = cachePromoteOpsRegex.FindStringSubmatch(clientStr)
	if len(matched) == 2 {
		v, err := strconv.Atoi(matched[1])
		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(c.CachePromoteIOOps, prometheus.GaugeValue, float64(v))
	}
	return nil
}

// Describe sends all the descriptions of individual metrics of ClusterHealthCollector
// to the provided prometheus channel.
func (c *ClusterHealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.RbdMirrorUp

	for _, metric := range c.descriptorList() {
		ch <- metric
	}
}

// Collect sends all the collected metrics to the provided prometheus channel.
// It requires the caller to handle synchronization.
func (c *ClusterHealthCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Debug("collecting cluster health metrics")
	if err := c.collect(ch); err != nil {
		c.logger.WithError(err).Error("error collecting cluster health metrics " + err.Error())
	}

	c.logger.Debug("collecting cluster recovery/client I/O metrics")
	if err := c.collectRecoveryClientIO(ch); err != nil {
		c.logger.WithError(err).Error("error collecting cluster recovery/client I/O metrics")
	}

	for _, metric := range c.collectorsList() {
		metric.Collect(ch)
	}
}
