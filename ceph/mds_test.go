package ceph

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMDSStats(t *testing.T) {
	for _, tt := range []struct {
		input     []byte
		version   string
		reMatch   []*regexp.Regexp
		reUnmatch []*regexp.Regexp
	}{
		{
			input: []byte(`
			{
				"fsmap": {
				  "filesystems": [
					{
					"mdsmap": {
					  "max_mds": 2,
					  "info": {
						"gid_19706291": {
						  "gid": 1970629,
						  "name": "MDS-daemonC",
						  "rank": 1,
						  "state": "up:active"
						},
						  "gid_19706292": {
							"gid": 1970629,
							"name": "MDS-daemonD",
							"rank": 2,
							"state": "up:standby-replay"
						  }
						},
					  "data_pools": [
						1
					  ],
					  "metadata_pool": 2,
					  "enabled": true,
					  "fs_name": "cephfs-1",
					  "balancer": "",
					  "standby_count_wanted": 1
					},
					"id": 2
				  },
					{
					  "mdsmap": {
						"max_mds": 2,
						"info": {
						  "gid_19706293": {
							"gid": 1970629,
							"name": "MDS-daemonA",
							"rank": 1,
							"state": "up:active"
						  },
							"gid_19706294": {
							  "gid": 1970629,
							  "name": "MDS-daemonB",
							  "rank": 2,
							  "state": "up:standby-replay"
							}
						  },
						"data_pools": [
						  1
						],
						"metadata_pool": 2,
						"enabled": true,
						"fs_name": "cephfs-2",
						"balancer": "",
						"standby_count_wanted": 1
					  },
					  "id": 2
					}
			]
		}
	}
`),
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_mds_daemon_state{cluster="ceph",fs="cephfs-1",name="MDS-daemonC",rank="1",state="up:active"} 1`),
				regexp.MustCompile(`ceph_mds_daemon_state{cluster="ceph",fs="cephfs-1",name="MDS-daemonD",rank="2",state="up:standby-replay"} 1`),
				regexp.MustCompile(`ceph_mds_daemon_state{cluster="ceph",fs="cephfs-2",name="MDS-daemonA",rank="1",state="up:active"} 1`),
				regexp.MustCompile(`ceph_mds_daemon_state{cluster="ceph",fs="cephfs-2",name="MDS-daemonB",rank="2",state="up:standby-replay"} 1`),
			},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, "{}")

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			e.cc = map[string]versionedCollector{
				"mds": NewMDSCollector(e, false),
			}

			e.cc["mds"].(*MDSCollector).runMDSStatFn = func(_ context.Context, cluster, user string) ([]byte, error) {
				if tt.input != nil {
					return tt.input, nil
				}
				return nil, errors.New("fake error")
			}

			err := prometheus.Register(e)
			require.NoError(t, err)
			defer prometheus.Unregister(e)

			server := httptest.NewServer(promhttp.Handler())
			defer server.Close()

			resp, err := http.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			buf, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			for _, re := range tt.reMatch {
				require.True(t, re.Match(buf))
			}

			for _, re := range tt.reUnmatch {
				require.False(t, re.Match(buf))
			}
		}()
	}
}

func TestMDSBlockedOps(t *testing.T) {
	for _, tt := range []struct {
		mdsStat      []byte
		healthDetail []byte
		blockedOps   []byte
		mdsStatus    []byte
		version      string
		reMatch      []*regexp.Regexp
		reUnmatch    []*regexp.Regexp
	}{
		{
			mdsStat: []byte(`
			{
				"fsmap": {
				  "filesystems": [
					{
					"mdsmap": {
					  "max_mds": 2,
					  "info": {
						"gid_19706291": {
						  "gid": 1970629,
						  "name": "MDS-daemonC",
						  "rank": 1,
						  "state": "up:active"
						},
						  "gid_19706292": {
							"gid": 1970629,
							"name": "MDS-daemonD",
							"rank": 2,
							"state": "up:standby-replay"
						  }
						},
					  "data_pools": [
						1
					  ],
					  "metadata_pool": 2,
					  "enabled": true,
					  "fs_name": "cephfs-1",
					  "balancer": "",
					  "standby_count_wanted": 1
					},
					"id": 2
				  },
					{
					  "mdsmap": {
						"max_mds": 2,
						"info": {
						  "gid_19706293": {
							"gid": 1970629,
							"name": "MDS-daemonA",
							"rank": 1,
							"state": "up:active"
						  },
							"gid_19706294": {
							  "gid": 1970629,
							  "name": "MDS-daemonB",
							  "rank": 2,
							  "state": "up:standby-replay"
							}
						  },
						"data_pools": [
						  1
						],
						"metadata_pool": 2,
						"enabled": true,
						"fs_name": "cephfs-2",
						"balancer": "",
						"standby_count_wanted": 1
					  },
					  "id": 2
					}
			]
		}
	}
`),
			healthDetail: []byte(`
			{
				"status": "HEALTH_WARN",
				"checks": {
					"MDS_SLOW_REQUEST": {
						"severity": "HEALTH_WARN",
						"summary": {
							"message": "1 MDSs report slow requests",
							"count": 1
						},
						"detail": [
							{
								"message": "mds.nodeA(mds.1): 1 slow requests are blocked > 30 secs"
							}
						],
						"muted": false
					},
					"RECENT_CRASH": {
						"severity": "HEALTH_WARN",
						"summary": {
							"message": "1 daemons have recently crashed",
							"count": 1
						},
						"detail": [
							{
								"message": "mds.nodeB crashed on host nodeB at 2024-02-13T22:11:00.200261Z"
							}
						],
						"muted": false
					}
				}
			}
`),
			blockedOps: []byte(`
			{
				"ops": [
					{
						"description": "client_request(client.20074182:344151 rmdir #0x10000000030/8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})",
						"initiated_at": "2024-02-13T22:11:00.197241+0000",
						"age": 148692.11557664501,
						"duration": 148692.11559661099,
						"type_data": {
							"flag_point": "cleaned up request",
							"reqid": "client.20074182:344151",
							"op_type": "client_request",
							"client_info": {
								"client": "client.20074182",
								"tid": 344151
							},
							"events": [
								{
									"time": "2024-02-13T22:11:00.197241+0000",
									"event": "initiated"
								},
								{
									"time": "2024-02-13T22:11:00.197241+0000",
									"event": "throttled"
								},
								{
									"time": "2024-02-13T22:11:00.197241+0000",
									"event": "header_read"
								},
								{
									"time": "2024-02-13T22:11:00.197247+0000",
									"event": "all_read"
								},
								{
									"time": "2024-02-13T22:11:00.197250+0000",
									"event": "dispatched"
								},
								{
									"time": "2024-02-13T22:11:00.197331+0000",
									"event": "failed to wrlock, waiting"
								},
								{
									"time": "2024-02-13T22:11:00.197476+0000",
									"event": "failed to xlock, waiting"
								},
								{
									"time": "2024-02-13T22:11:00.197548+0000",
									"event": "acquired locks"
								},
								{
									"time": "2024-02-14T07:47:59.551074+0000",
									"event": "killing request"
								},
								{
									"time": "2024-02-14T07:47:59.551135+0000",
									"event": "cleaned up request"
								}
							]
						}
					}
				],
				"complaint_time": 30,
				"num_blocked_ops": 1
			}
`),
			mdsStatus: []byte(`
			{
				"cluster_fsid": "xxxx",
				"whoami": 1,
				"id": 20004801,
				"want_state": "up:active",
				"state": "up:active",
				"fs_name": "fsA",
				"rank_uptime": 160259.41429652399,
				"mdsmap_epoch": 881925,
				"osdmap_epoch": 222331815,
				"osdmap_epoch_barrier": 222331815,
				"uptime": 163199.411784772
			}
`),
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_mds_blocked_ops{cluster="ceph",flag_point="cleaned up request",fs="fsA",fs_optype="rmdir",inode="0x10000000030",name="mds.nodeA",optype="client_request",state="up:active"} 1`),
			},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, "{}")

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			mdsc := NewMDSCollector(e, false)
			mdsc.runCephHealthDetailFn = func(_ context.Context, cluster, user string) ([]byte, error) {
				if tt.healthDetail != nil {
					return tt.healthDetail, nil
				}
				return nil, errors.New("fake error")
			}
			mdsc.runBlockedOpsCheckFn = func(_ context.Context, cluster, user, mds string) ([]byte, error) {
				if tt.blockedOps != nil {
					return tt.blockedOps, nil
				}
				return nil, errors.New("fake error")
			}
			mdsc.runMDSStatusFn = func(_ context.Context, cluster, user, mds string) ([]byte, error) {
				if tt.mdsStatus != nil {
					return tt.mdsStatus, nil
				}
				return nil, errors.New("fake error")
			}
			mdsc.runMDSStatFn = func(_ context.Context, cluster, user string) ([]byte, error) {
				if tt.mdsStat != nil {
					return tt.mdsStat, nil
				}
				return nil, errors.New("fake error")
			}

			e.cc = map[string]versionedCollector{
				"mds": mdsc,
			}

			err := prometheus.Register(e)
			require.NoError(t, err)
			defer prometheus.Unregister(e)

			server := httptest.NewServer(promhttp.Handler())
			defer server.Close()

			resp, err := http.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			buf, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			for _, re := range tt.reMatch {
				require.True(t, re.Match(buf))
			}

			for _, re := range tt.reUnmatch {
				require.False(t, re.Match(buf))
			}
		}()
	}
}

func TestExtractOpFromDescription(t *testing.T) {
	for _, tt := range []struct {
		input  string
		errMsg string
		opd    *opDesc
	}{
		{
			input:  "client_request(client.20001974182:344151 rmdir #0x10000000030/72a26231-ac24-4f69-9350-8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})",
			errMsg: "",
			opd: &opDesc{
				fsOpType: "rmdir",
				inode:    "0x10000000030",
				clientID: "20001974182",
			},
		},
		{
			input:  "client_request(client.20001974182:344151rmdir#0x10000000030ZZZZ/72a26231-ac24-4f69-9350-8ebc5444c9ea2024-02-13T22:11:00.196767+0000caller_uid=0, caller_gid=0{})",
			errMsg: `invalid fs description`,
			opd:    nil,
		},
		{
			input:  "client_request(client.20001974182:344151 rmdir #0x10000000030ZZZZ/72a26231-ac24-4f69-9350-8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})",
			errMsg: `invalid inode, expected hex instead got "0x10000000030ZZZZ": strconv.ParseUint: parsing "10000000030ZZZZ": invalid syntax`,
			opd:    nil,
		},
		{
			input:  "client_request[client.20001974182:344151 rmdir #0x10000000030/72a26231-ac24-4f69-9350-8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})",
			errMsg: `invalid client request format`,
			opd:    nil,
		},
		{
			input:  "client_request(client.20001974182/344151 rmdir #0x10000000030/72a26231-ac24-4f69-9350-8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})",
			errMsg: `invalid client id format`,
			opd:    nil,
		},
		{
			input:  "client_request(client|20001974182:344151 rmdir #0x10000000030/72a26231-ac24-4f69-9350-8ebc5444c9ea 2024-02-13T22:11:00.196767+0000 caller_uid=0, caller_gid=0{})",
			errMsg: `nvalid client id string`,
			opd:    nil,
		},
	} {
		opd, err := extractOpFromDescription(tt.input)
		if tt.errMsg != "" {
			require.NotNil(t, err)
		}
		if tt.errMsg == "" {
			require.NoError(t, err)
		}
		if err != nil {
			require.ErrorContains(t, err, tt.errMsg)
		}
		if tt.opd != nil {
			require.Equal(t, tt.opd, opd)
		}
	}
}
