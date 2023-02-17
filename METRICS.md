# Metrics Collected

Ceph exporter implements multiple collectors:

## Cluster usage

General cluster level data usage.

Labels:
- `cluster`: cluster name

Metrics:
- `ceph_cluster_capacity_bytes`: Total capacity of the cluster
- `ceph_cluster_used_bytes`: Capacity of the cluster currently in use
- `ceph_cluster_available_bytes`: Available space within the cluster

## Pool usage

Per-pool usage data

Labels:
- `cluster`: cluster name
- `pool`: pool name

Metrics:
 - `ceph_pool_used_bytes`: Capacity of the pool that is currently under use
 - `ceph_pool_raw_used_bytes`: Raw capacity of the pool that is currently under use, this factors in the size
 - `ceph_pool_available_bytes`: Free space for the pool
 - `ceph_pool_percent_used`: Percentage of the capacity available to this pool that is used by this pool
 - `ceph_pool_objects_total`: Total no. of objects allocated within the pool
 - `ceph_pool_dirty_objects_total`: Total no. of dirty objects in a cache-tier pool
 - `ceph_pool_unfound_objects_total`: Total no. of unfound objects for the pool
 - `ceph_pool_read_total`: Total read I/O calls for the pool
 - `ceph_pool_read_bytes_total`: Total read throughput for the pool
 - `ceph_pool_write_total`: Total write I/O calls for the pool
 - `ceph_pool_write_bytes_total`: Total write throughput for the pool

## Pool info

General pool information

Labels:
- `cluster`: cluster name
- `pool`: pool name
- `root`: CRUSH root of the pool
- `profile`: `replicated` or EC profile being used

Metrics:
- `ceph_pool_pg_num`: The total count of PGs alotted to a pool
- `ceph_pool__pgp_num`: The total count of PGs alotted to a pool and used for placements
- `ceph_pool_min_size`: Minimum number of copies or chunks of an object that need to be present for active I/O
- `ceph_pool_size`: Total copies or chunks of an object that need to be present for a healthy cluster
- `ceph_pool_quota_max_bytes`: Maximum amount of bytes of data allowed in a pool
- `ceph_pool_quota_max_objects`: Maximum amount of RADOS objects allowed in a pool
- `ceph_pool_stripe_width`: Stripe width of a RADOS object in a pool
- `ceph_pool_expansion_factor`: Data expansion multiplier for a pool

## Cluster health

Cluster health metrics

Labels:
 - `cluster`: cluster name

Metrics:
- `ceph_health_status`: Health status of Cluster, can vary only between 3 states (err:2, warn:1, ok:0)
- `ceph_health_status_interp`: Health status of Cluster, can vary only between 4 states (err:3, critical_warn:2, soft_warn:1, ok:0)
- `ceph_mons_down`: Count of Mons that are in DOWN state
- `ceph_total_pgs`: Total no. of PGs in the cluster
- `ceph_pg_state`: State of PGs in the cluster
- `ceph_active_pgs`: No. of active PGs in the cluster
- `ceph_scrubbing_pgs`: No. of scrubbing PGs in the cluster
- `ceph_deep_scrubbing_pgs`: No. of deep scrubbing PGs in the cluster
- `ceph_recovering_pgs`: No. of recovering PGs in the cluster
- `ceph_recovery_wait_pgs`: No. of PGs in the cluster with recovery_wait state
- `ceph_backfilling_pgs`: No. of backfilling PGs in the cluster
- `ceph_backfill_wait_pgs`: No. of PGs in the cluster with backfill_wait state
- `ceph_forced_recovery_pgs`: No. of PGs in the cluster with forced_recovery state
- `ceph_forced_backfill_pgs`: No. of PGs in the cluster with forced_backfill state
- `ceph_down_pgs`: No. of PGs in the cluster in down state
- `ceph_incomplete_pgs`: No. of PGs in the cluster in incomplete state
- `ceph_inconsistent_pgs`: No. of PGs in the cluster in inconsistent state
- `ceph_snaptrim_pgs`: No. of snaptrim PGs in the cluster
- `ceph_snaptrim_wait_pgs`: No. of PGs in the cluster with snaptrim_wait state
- `ceph_repairing_pgs`: No. of PGs in the cluster with repair state
- `ceph_slow_requests`: No. of slow requests/slow ops
- `ceph_degraded_pgs`: No. of PGs in a degraded state
- `ceph_stuck_degraded_pgs`: No. of PGs stuck in a degraded state
- `ceph_unclean_pgs`: No. of PGs in an unclean state
- `ceph_stuck_unclean_pgs`: No. of PGs stuck in an unclean state
- `ceph_undersized_pgs`: No. of undersized PGs in the cluster
- `ceph_stuck_undersized_pgs`: No. of stuck undersized PGs in the cluster
- `ceph_stale_pgs`: No. of stale PGs in the cluster
- `ceph_stuck_stale_pgs`: No. of stuck stale PGs in the cluster
- `ceph_peering_pgs`: No. of peering PGs in the cluster
- `ceph_degraded_objects`: No. of degraded objects across all PGs, includes replicas
- `ceph_misplaced_objects`: No. of misplaced objects across all PGs, includes replicas
- `ceph_misplaced_ratio`: ratio of misplaced objects to total objects
- `ceph_new_crash_reports`: Number of new crash reports available
- `ceph_osds_too_many_repair`: Number of OSDs with too many repaired reads
- `ceph_cluster_objects`: No. of rados objects within the cluster
- `ceph_osd_map_flags`: A metric for all OSDMap flags
- `ceph_osds_down`: Count of OSDs that are in DOWN state
- `ceph_osds_up`: Count of OSDs that are in UP state
- `ceph_osds_in`: Count of OSDs that are in IN state and available to serve requests
- `ceph_osds`: Count of total OSDs in the cluster
- `ceph_pgs_remapped`: No. of PGs that are remapped and incurring cluster-wide movement
- `ceph_recovery_io_bytes`: Rate of bytes being recovered in cluster per second
- `ceph_recovery_io_keys`: Rate of keys being recovered in cluster per second
- `ceph_recovery_io_objects`: Rate of objects being recovered in cluster per second
- `ceph_client_io_read_bytes`: Rate of bytes being read by all clients per second
- `ceph_client_io_write_bytes`: Rate of bytes being written by all clients per second
- `ceph_client_io_ops`: Total client ops on the cluster measured per second
- `ceph_client_io_read_ops`: Total client read I/O ops on the cluster measured per second
- `ceph_client_io_write_ops`: Total client write I/O ops on the cluster measured per second
- `ceph_cache_flush_io_bytes`: Rate of bytes being flushed from the cache pool per second
- `ceph_cache_evict_io_bytes`: Rate of bytes being evicted from the cache pool per second
- `ceph_cache_promote_io_ops`: Total cache promote operations measured per second
- `ceph_mgrs_active`: Count of active mgrs, can be either 0 or 1
- `ceph_mgrs`: Total number of mgrs, including standbys
- `ceph_rbd_mirror_up`: Alive rbd-mirror daemons

## Ceph monitor

Ceph Monitor metrics

Labels:
- `cluster`: cluster name
- `daemon`: daemon name. `ceph_versions` and `ceph_features` only
- `release`, `features`: ceph feature name and feature flag. `ceph_features` only
- `version_tag`, `sha1`, `release_name`:  ceph version infortmation. `ceph_features` only

Metrics:
- `ceph_monitor_capacity_bytes`: Total storage capacity of the monitor node
- `ceph_monitor_used_bytes`: Storage of the monitor node that is currently allocated for use
- `ceph_monitor_avail_bytes`: Total unused storage capacity that the monitor node has left
- `ceph_monitor_avail_percent`: Percentage of total unused storage capacity that the monitor node has left
- `ceph_monitor_store_capacity_bytes`: Total capacity of the FileStore backing the monitor daemon
- `ceph_monitor_store_sst_bytes`: Capacity of the FileStore used only for raw SSTs
- `ceph_monitor_store_log_bytes`: Capacity of the FileStore used only for logging
- `ceph_monitor_store_misc_bytes`: Capacity of the FileStore used only for storing miscellaneous information
- `ceph_monitor_clock_skew_seconds`: Clock skew the monitor node is incurring
- `ceph_monitor_latency_seconds`: Latency the monitor node is incurring
- `ceph_monitor_quorum_count`: he total size of the monitor quorum
- `ceph_versions`: Counts of current versioned daemons, parsed from `ceph versions`
- `ceph_features`: Counts of current client features, parsed from `ceph features`

## OSD collector

OSD level metrics

Labels:
- `cluster`: cluster name
- `osd`: OSD id
- `device_class`: CRUSH device class
- `host`: CRUSH host the OSD is in
- `rack`: CRUSH rack the OSD is in
- `root`: CRUSH root the OSD is in
- `pgid`: PG id for recovery related metrics

Metrics:
- `ceph_osd_crush_weight`: OSD Crush Weight
- `ceph_osd_depth`: OSD Depth
- `ceph_osd_reweight`: OSD Reweight
- `ceph_osd_bytes`: OSD Total Bytes
- `ceph_osd_used_bytes`: OSD Used Storage in Bytes
- `ceph_osd_avail_bytes`: OSD Available Storage in Bytes
- `ceph_osd_utilization`: OSD Utilization
- `ceph_osd_variance`: OSD Variance
- `ceph_osd_pgs`: OSD Placement Group Count
- `ceph_osd_pg_upmap_items_total`: OSD PG-Upmap Exception Table Entry Count
- `ceph_osd_total_bytes`: OSD Total Storage Bytes
- `ceph_osd_total_used_bytes`: OSD Total Used Storage Bytes
- `ceph_osd_total_avail_bytes`: OSD Total Available Storage Bytes
- `ceph_osd_average_utilization`: OSD Average Utilization
- `ceph_osd_perf_commit_latency_seconds`: OSD Perf Commit Latency
- `ceph_osd_perf_apply_latency_seconds`: OSD Perf Apply Latency
- `ceph_osd_in`: OSD In Status
- `ceph_osd_up`: OSD Up Status
- `ceph_osd_full_ratio`: OSD Full Ratio Value
- `ceph_osd_near_full_ratio`: OSD Near Full Ratio Value
- `ceph_osd_backfill_full_ratio`: OSD Backfill Full Ratio Value
- `ceph_osd_full`: OSD Full Status
- `ceph_osd_near_full`: OSD Near Full Status
- `ceph_osd_backfill_full`: OSD Backfill Full Status
- `ceph_osd_down`: Number of OSDs down in the cluster
- `ceph_osd_scrub_state`: State of OSDs involved in a scrub
- `ceph_pg_objects_recovered`: Number of objects recovered in a PG
- `ceph_osd_objects_backfilled`: Average number of objects backfilled in an OSD
- `ceph_pg_oldest_inactive`: The amount of time in seconds that the oldest PG has been inactive for

## Crash collector

Ceph crash daemon related metrics

Labels:
- `cluster`: cluster name

Metrics:
- `ceph_crash_reports`: Count of crashes reports per daemon, according to `ceph crash ls`

## RBD Mirror collector

Ceph RBD mirror health collector

Labels:
- `cluster`: cluster name

Metrics:
- `ceph_rbd_mirror_pool_status`: Health status of rbd-mirror, can vary only between 3 states (err:2, warn:1, ok:0)
- `ceph_rbd_mirror_pool_daemon_status`: Health status of rbd-mirror daemons, can vary only between 3 states (err:2, warn:1, ok:0)
- `ceph_rbd_mirror_pool_image_status`: "Health status of rbd-mirror images, can vary only between 3 states (err:2, warn:1, ok:0)

## RGW collector

RGW related metrics. Only enabled if `RGW_MODE={1,2}` is set.

Labels:
- `cluster`: cluster name

Metrics:
- `ceph_rgw_gc_active_tasks`: RGW GC active task count
- `ceph_rgw_gc_active_objects`: RGW GC active object count
- `ceph_rgw_gc_pending_tasks`: RGW GC pending task count
- `ceph_rgw_gc_pending_objects`: RGW GC pending object count
