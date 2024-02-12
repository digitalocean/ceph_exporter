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
