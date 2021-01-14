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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/digitalocean/ceph_exporter/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMonitorCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		regexes []*regexp.Regexp
	}{
		{
			`
{
    "health": {
        "health": {
            "health_services": [
                {
                    "mons": [
                        {
                            "name": "test-mon01",
                            "kb_total": 412718256,
                            "kb_used": 1812852,
                            "kb_avail": 389917500,
                            "avail_percent": 94,
                            "last_updated": "2015-12-28 15:54:03.763348",
                            "store_stats": {
                                "bytes_total": 1781282079,
                                "bytes_sst": 1,
                                "bytes_log": 609694,
                                "bytes_misc": 1780672385,
                                "last_updated": "0.000000"
                            },
                            "health": "HEALTH_OK"
                        },
                        {
                            "name": "test-mon02",
                            "kb_total": 412718256,
                            "kb_used": 1875304,
                            "kb_avail": 389855048,
                            "avail_percent": 94,
                            "last_updated": "2015-12-28 15:53:53.808657",
                            "store_stats": {
                                "bytes_total": 1844348214,
                                "bytes_sst": 2,
                                "bytes_log": 871605,
                                "bytes_misc": 1843476609,
                                "last_updated": "0.000000"
                            },
                            "health": "HEALTH_OK"
                        },
                        {
                            "name": "test-mon03",
                            "kb_total": 412718256,
                            "kb_used": 2095356,
                            "kb_avail": 389634996,
                            "avail_percent": 94,
                            "last_updated": "2015-12-28 15:53:06.292749",
                            "store_stats": {
                                "bytes_total": 2069468587,
                                "bytes_sst": 3,
                                "bytes_log": 871605,
                                "bytes_misc": 2068596982,
                                "last_updated": "0.000000"
                            },
                            "health": "HEALTH_OK"
                        },
                        {
                            "name": "test-mon04",
                            "kb_total": 412718256,
                            "kb_used": 1726276,
                            "kb_avail": 390004076,
                            "avail_percent": 94,
                            "last_updated": "2015-12-28 15:53:10.770775",
                            "store_stats": {
                                "bytes_total": 1691972147,
                                "bytes_sst": 4,
                                "bytes_log": 871605,
                                "bytes_misc": 1691100542,
                                "last_updated": "0.000000"
                            },
                            "health": "HEALTH_OK"
                        },
                        {
                            "name": "test-mon05",
                            "kb_total": 412718256,
                            "kb_used": 1883228,
                            "kb_avail": 389847124,
                            "avail_percent": 94,
                            "last_updated": "2015-12-28 15:53:11.407033",
                            "store_stats": {
                                "bytes_total": 1852485942,
                                "bytes_sst": 5,
                                "bytes_log": 871605,
                                "bytes_misc": 1851614337,
                                "last_updated": "0.000000"
                            },
                            "health": "HEALTH_OK"
                        }
                    ]
                }
            ]
        },
        "timechecks": {
            "epoch": 70,
            "round": 3362,
            "round_status": "finished",
            "mons": [
                {
                    "name": "test-mon01",
                    "skew": 0.000000,
                    "latency": 0.000000,
                    "health": "HEALTH_OK"
                },
                {
                    "name": "test-mon02",
                    "skew": -0.000002,
                    "latency": 0.000815,
                    "health": "HEALTH_OK"
                },
                {
                    "name": "test-mon03",
                    "skew": -0.000002,
                    "latency": 0.000829,
                    "health": "HEALTH_OK"
                },
                {
                    "name": "test-mon04",
                    "skew": -0.000019,
                    "latency": 0.000609,
                    "health": "HEALTH_OK"
                },
                {
                    "name": "test-mon05",
                    "skew": -0.000628,
                    "latency": 0.000659,
                    "health": "HEALTH_OK"
                }
            ]
        },
        "summary": [],
        "overall_status": "HEALTH_OK",
        "detail": []
    },
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
			[]*regexp.Regexp{
				regexp.MustCompile(`ceph_monitor_avail_bytes{cluster="ceph",monitor="test-mon01"} 3.9927552e`),
				regexp.MustCompile(`ceph_monitor_avail_bytes{cluster="ceph",monitor="test-mon02"} 3.99211569152e`),
				regexp.MustCompile(`ceph_monitor_avail_bytes{cluster="ceph",monitor="test-mon03"} 3.98986235904e`),
				regexp.MustCompile(`ceph_monitor_avail_bytes{cluster="ceph",monitor="test-mon04"} 3.99364173824`),
				regexp.MustCompile(`ceph_monitor_avail_bytes{cluster="ceph",monitor="test-mon05"} 3.99203454976e`),
				regexp.MustCompile(`ceph_monitor_avail_percent{cluster="ceph",monitor="test-mon01"} 94`),
				regexp.MustCompile(`ceph_monitor_avail_percent{cluster="ceph",monitor="test-mon02"} 94`),
				regexp.MustCompile(`ceph_monitor_avail_percent{cluster="ceph",monitor="test-mon03"} 94`),
				regexp.MustCompile(`ceph_monitor_avail_percent{cluster="ceph",monitor="test-mon04"} 94`),
				regexp.MustCompile(`ceph_monitor_avail_percent{cluster="ceph",monitor="test-mon05"} 94`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon01"} 0`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon02"} -2e-06`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon03"} -2e-06`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon04"} -1.9e-05`),
				regexp.MustCompile(`ceph_monitor_clock_skew_seconds{cluster="ceph",monitor="test-mon05"} -0.000628`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon01"} 0`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon02"} 0.000815`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon03"} 0.000829`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon04"} 0.000609`),
				regexp.MustCompile(`ceph_monitor_latency_seconds{cluster="ceph",monitor="test-mon05"} 0.000659`),
				regexp.MustCompile(`ceph_monitor_quorum_count{cluster="ceph"} 5`),
				regexp.MustCompile(`ceph_monitor_store_log_bytes{cluster="ceph",monitor="test-mon01"} 609694`),
				regexp.MustCompile(`ceph_monitor_store_log_bytes{cluster="ceph",monitor="test-mon02"} 871605`),
				regexp.MustCompile(`ceph_monitor_store_log_bytes{cluster="ceph",monitor="test-mon03"} 871605`),
				regexp.MustCompile(`ceph_monitor_store_log_bytes{cluster="ceph",monitor="test-mon04"} 871605`),
				regexp.MustCompile(`ceph_monitor_store_log_bytes{cluster="ceph",monitor="test-mon05"} 871605`),
				regexp.MustCompile(`ceph_monitor_store_misc_bytes{cluster="ceph",monitor="test-mon01"} 1.780672385e`),
				regexp.MustCompile(`ceph_monitor_store_misc_bytes{cluster="ceph",monitor="test-mon02"} 1.843476609e`),
				regexp.MustCompile(`ceph_monitor_store_misc_bytes{cluster="ceph",monitor="test-mon03"} 2.068596982e`),
				regexp.MustCompile(`ceph_monitor_store_misc_bytes{cluster="ceph",monitor="test-mon04"} 1.691100542e`),
				regexp.MustCompile(`ceph_monitor_store_misc_bytes{cluster="ceph",monitor="test-mon05"} 1.851614337e`),
				regexp.MustCompile(`ceph_monitor_store_sst_bytes{cluster="ceph",monitor="test-mon01"} 1`),
				regexp.MustCompile(`ceph_monitor_store_sst_bytes{cluster="ceph",monitor="test-mon02"} 2`),
				regexp.MustCompile(`ceph_monitor_store_sst_bytes{cluster="ceph",monitor="test-mon03"} 3`),
				regexp.MustCompile(`ceph_monitor_store_sst_bytes{cluster="ceph",monitor="test-mon04"} 4`),
				regexp.MustCompile(`ceph_monitor_store_sst_bytes{cluster="ceph",monitor="test-mon05"} 5`),
				regexp.MustCompile(`ceph_monitor_store_capacity_bytes{cluster="ceph",monitor="test-mon01"} 1.781282079e`),
				regexp.MustCompile(`ceph_monitor_store_capacity_bytes{cluster="ceph",monitor="test-mon02"} 1.844348214e`),
				regexp.MustCompile(`ceph_monitor_store_capacity_bytes{cluster="ceph",monitor="test-mon03"} 2.069468587e`),
				regexp.MustCompile(`ceph_monitor_store_capacity_bytes{cluster="ceph",monitor="test-mon04"} 1.691972147e`),
				regexp.MustCompile(`ceph_monitor_store_capacity_bytes{cluster="ceph",monitor="test-mon05"} 1.852485942e`),
				regexp.MustCompile(`ceph_monitor_capacity_bytes{cluster="ceph",monitor="test-mon01"} 4.22623494144e`),
				regexp.MustCompile(`ceph_monitor_capacity_bytes{cluster="ceph",monitor="test-mon02"} 4.22623494144e`),
				regexp.MustCompile(`ceph_monitor_capacity_bytes{cluster="ceph",monitor="test-mon03"} 4.22623494144e`),
				regexp.MustCompile(`ceph_monitor_capacity_bytes{cluster="ceph",monitor="test-mon04"} 4.22623494144e`),
				regexp.MustCompile(`ceph_monitor_capacity_bytes{cluster="ceph",monitor="test-mon05"} 4.22623494144e`),
				regexp.MustCompile(`ceph_monitor_used_bytes{cluster="ceph",monitor="test-mon01"} 1.856360448e`),
				regexp.MustCompile(`ceph_monitor_used_bytes{cluster="ceph",monitor="test-mon02"} 1.920311296e`),
				regexp.MustCompile(`ceph_monitor_used_bytes{cluster="ceph",monitor="test-mon03"} 2.145644544e`),
				regexp.MustCompile(`ceph_monitor_used_bytes{cluster="ceph",monitor="test-mon04"} 1.767706624e`),
				regexp.MustCompile(`ceph_monitor_used_bytes{cluster="ceph",monitor="test-mon05"} 1.928425472e`),
			},
		},
	} {
		func() {
			conn := &mocks.Conn{}
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			collector := NewMonitorCollector(conn, "ceph", logrus.New())
			err := prometheus.Register(collector)
			require.NoError(t, err)
			defer prometheus.Unregister(collector)

			server := httptest.NewServer(promhttp.Handler())
			defer server.Close()

			resp, err := http.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			buf, err := ioutil.ReadAll(resp.Body)
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
		reMatch []*regexp.Regexp
	}{
		{`
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
			[]*regexp.Regexp{
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
		{`
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew": "wrong!",
                        "latency": 0.000677,
                        "health": "HEALTH_OK"
                }
            }            
`,
			[]*regexp.Regexp{},
		},
		{`
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew": 0.000334,
                        "latency": "wrong!",
                        "health": "HEALTH_OK"
                }
            }            
`,
			[]*regexp.Regexp{},
		},
		{`
            {
                "time_skew_status": {
                    "test-mon01": {
                        "skew"::: "0.000334",
                        "latency"::: "0.000677",
                        "health": "HEALTH_OK"
                }
            }            
`,
			[]*regexp.Regexp{},
		},
	} {
		func() {
			conn := &mocks.Conn{}
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			collector := NewMonitorCollector(conn, "ceph", logrus.New())
			err := prometheus.Register(collector)
			require.NoError(t, err)
			defer prometheus.Unregister(collector)

			server := httptest.NewServer(promhttp.Handler())
			defer server.Close()

			resp, err := http.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			buf, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)

			for _, re := range tt.reMatch {
				require.True(t, re.Match(buf))
			}
		}()
	}
}
