//   Copyright 2024 DigitalOcean
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
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMonitorCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		version string
		regexes []*regexp.Regexp
	}{
		{
			input: `
{
    "fsid": "6C9BF03E-044E-4EEB-9C5F-145A54ECF7DB",
    "election_epoch": 70,
    "quorum": [
        0,
        1,
        2,
        3,
        4
    ],
    "monmap": {
        "epoch": 12,
        "fsid": "6C9BF03E-044E-4EEB-9C5F-145A54ECF7DB",
        "modified": "2015-11-25 07:58:56.388352",
        "created": "0.000000",
        "mons": [
            {
                "rank": 0,
                "name": "test-mon01",
                "addr": "10.123.1.25:6789\/0"
            },
            {
                "rank": 1,
                "name": "test-mon02",
                "addr": "10.123.1.26:6789\/0"
            },
            {
                "rank": 2,
                "name": "test-mon03",
                "addr": "10.123.2.25:6789\/0"
            },
            {
                "rank": 3,
                "name": "test-mon04",
                "addr": "10.123.2.26:6789\/0"
            },
            {
                "rank": 4,
                "name": "test-mon05",
                "addr": "10.123.2.27:6789\/0"
            }
        ]
    }
}
`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_monitor_quorum_count{cluster="ceph"} 5`),
			},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, "{}")
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			e.cc = map[string]versionedCollector{
				"mon": NewMonitorCollector(e),
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

			for _, re := range tt.regexes {
				require.True(t, re.Match(buf))
			}
		}()
	}
}

func TestMonitorTimeSyncStats(t *testing.T) {
	for _, tt := range []struct {
		input   string
		version string
		reMatch []*regexp.Regexp
	}{
		{
			input: `
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew": 0.000022,
                        "latency": 0.000677,
                        "health": "HEALTH_OK"
                    },
                    "test-mon02": {
                        "skew": 0.001051,
                        "latency": 0.000682,
                        "health": "HEALTH_OK"
                    },
                    "test-mon03": {
                        "skew": 0.003029,
                        "latency": 0.000582,
                        "health": "HEALTH_OK"
                    },
                    "test-mon04": {
                        "skew": 0.000330,
                        "latency": 0.000667,
                        "health": "HEALTH_OK"
                    },
                    "test-mon05": {
                        "skew": 0.003682,
                        "latency": 0.000667,
                        "health": "HEALTH_OK"
                    }
                },
                "timechecks": {
                    "epoch": 84,
                    "round": 69600,
                    "round_status": "finished"
                }
            }            
`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon01"} 2.2e\-05`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon02"} 0.001051`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon03"} 0.003029`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon04"} 0.00033`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon05"} 0.003682`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon01"} 0.000677`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon02"} 0.000682`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon03"} 0.000582`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon04"} 0.000667`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon05"} 0.000667`),
			},
		},
		{
			input: `
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew": "wrong!",
                        "latency": 0.000677,
                        "health": "HEALTH_OK"
                }
            }            
`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{},
		},
		{
			input: `
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew": 0.000334,
                        "latency": "wrong!",
                        "health": "HEALTH_OK"
                }
            }            
`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{},
		},
		{
			input: `
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew"::: "0.000334",
                        "latency"::: "0.000677",
                        "health": "HEALTH_OK"
                }
            }            
`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, "{}")
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			e.cc = map[string]versionedCollector{
				"mon": NewMonitorCollector(e),
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
		}()
	}
}

func TestMonitorCephVersions(t *testing.T) {
	for _, tt := range []struct {
		input   string
		version string
		reMatch []*regexp.Regexp
	}{
		{
			input: `
{
    "mon": {
        "ceph version 12.2.13 (584a20eb0237c657dc0567da126be145106aa47e) luminous (stable)": 5
    },
    "mgr": {
        "ceph version 12.2.13 (584a20eb0237c657dc0567da126be145106aa47e) luminous (stable)": 5
    },
    "osd": {
        "ceph version 12.2.11-3-gc5b1fd5 (c5b1fd521188cccdedcf6f98c40e6a02286042f2) luminous (stable)": 450
    },
    "mds": {},
    "rgw": {
        "ceph version 12.2.5-8-g58a2283 (58a2283da6a62d2cc1600d4a9928a0799d63c7c9) luminous (stable)": 4
    },
    "overall": {
        "ceph version 12.2.11-3-gc5b1fd5 (c5b1fd521188cccdedcf6f98c40e6a02286042f2) luminous (stable)": 450,
        "ceph version 12.2.13 (584a20eb0237c657dc0567da126be145106aa47e) luminous (stable)": 10,
        "ceph version 12.2.5-8-g58a2283 (58a2283da6a62d2cc1600d4a9928a0799d63c7c9) luminous (stable)": 4
    }
}
`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_versions{cluster="ceph",daemon="mon",release_name="luminous",sha1="584a20eb0237c657dc0567da126be145106aa47e",version_tag="12.2.13"} 5`),
				regexp.MustCompile(`ceph_versions{cluster="ceph",daemon="rgw",release_name="luminous",sha1="58a2283da6a62d2cc1600d4a9928a0799d63c7c9",version_tag="12.2.5-8-g58a2283"} 4`),
			},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, tt.input)
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			e.cc = map[string]versionedCollector{
				"mon": NewMonitorCollector(e),
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
		}()
	}
}

func TestMonitorCephFeatures(t *testing.T) {
	for _, tt := range []struct {
		input   string
		version string
		reMatch []*regexp.Regexp
	}{
		{
			input: `
{
    "mon": [
        {
            "features": "0x3ffddff8ffecffff",
            "release": "luminous",
            "num": 5
        }
    ],
    "osd": [
        {
            "features": "0x3ffddff8ffecffff",
            "release": "luminous",
            "num": 320
        }
    ],
    "client": [
        {
            "features": "0x3ffddff8eeacfffb",
            "release": "luminous",
            "num": 2
        },
        {
            "features": "0x3ffddff8ffacffff",
            "release": "luminous",
            "num": 53
        },
        {
            "features": "0x3ffddff8ffecffff",
            "release": "luminous",
            "num": 4
        }
    ],
    "mgr": [
        {
            "features": "0x3ffddff8ffecffff",
            "release": "luminous",
            "num": 5
        }
    ]
}
		`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_features{cluster="ceph",daemon="client",features="0x3ffddff8ffacffff",release="luminous"} 53`),
			},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, "{}")
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			e.cc = map[string]versionedCollector{
				"mon": NewMonitorCollector(e),
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
		}()
	}
}
