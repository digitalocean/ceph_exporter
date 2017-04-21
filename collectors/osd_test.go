package collectors

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestOSDCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		regexes []*regexp.Regexp
	}{
		{
			input: `
{
    "nodes": [
        {
            "id": 0,
            "name": "osd.0",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.010391,
            "depth": 2,
            "reweight": 1.000000,
            "kb": 11150316,
            "kb_used": 40772,
            "kb_avail": 11109544,
            "utilization": 0.365658,
            "var": 1.053676,
            "pgs": 283
        },
        {
            "id": 2,
            "name": "osd.2",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.010391,
            "depth": 2,
            "reweight": 1.000000,
            "kb": 11150316,
            "kb_used": 36712,
            "kb_avail": 11113604,
            "utilization": 0.329246,
            "var": 0.948753,
            "pgs": 162
        },
        {
            "id": 1,
            "name": "osd.1",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.010391,
            "depth": 2,
            "reweight": 1.000000,
            "kb": 11150316,
            "kb_used": 40512,
            "kb_avail": 11109804,
            "utilization": 0.363326,
            "var": 1.046957,
            "pgs": 279
        },
        {
            "id": 3,
            "name": "osd.3",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.010391,
            "depth": 2,
            "reweight": 1.000000,
            "kb": 11150316,
            "kb_used": 36784,
            "kb_avail": 11113532,
            "utilization": 0.329892,
            "var": 0.950614,
            "pgs": 164
        },
        {
            "id": 4,
            "name": "osd.4",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.010391,
            "depth": 2,
            "reweight": 0,
            "kb": 0,
            "kb_used": 0,
            "kb_avail": 0,
            "utilization": -nan,
            "var": -nan,
            "pgs": 0
        }
    ],
    "stray": [],
    "summary": {
        "total_kb": 44601264,
        "total_kb_used": 154780,
        "total_kb_avail": 44446484,
        "average_utilization": 0.347031,
        "min_var": 0.948753,
        "max_var": 1.053676,
        "dev": 0.017482
    }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",osd="osd.0"} 0.010391`),
				regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",osd="osd.1"} 0.010391`),
				regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",osd="osd.2"} 0.010391`),
				regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",osd="osd.3"} 0.010391`),
				regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",osd="osd.4"} 0.010391`),
				regexp.MustCompile(`ceph_osd_depth{cluster="ceph",osd="osd.0"} 2`),
				regexp.MustCompile(`ceph_osd_depth{cluster="ceph",osd="osd.1"} 2`),
				regexp.MustCompile(`ceph_osd_depth{cluster="ceph",osd="osd.2"} 2`),
				regexp.MustCompile(`ceph_osd_depth{cluster="ceph",osd="osd.3"} 2`),
				regexp.MustCompile(`ceph_osd_depth{cluster="ceph",osd="osd.4"} 2`),
				regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",osd="osd.0"} 1`),
				regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",osd="osd.1"} 1`),
				regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",osd="osd.2"} 1`),
				regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",osd="osd.3"} 1`),
				regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",osd="osd.0"} 1.1150316e`),
				regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",osd="osd.1"} 1.1150316e`),
				regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",osd="osd.2"} 1.1150316e`),
				regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",osd="osd.3"} 1.1150316e`),
				regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",osd="osd.0"} 4.0772e`),
				regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",osd="osd.1"} 4.0512e`),
				regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",osd="osd.2"} 3.6712e`),
				regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",osd="osd.3"} 3.6784e`),
				regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",osd="osd.0"} 1.1109544e`),
				regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",osd="osd.1"} 1.1109804e`),
				regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",osd="osd.2"} 1.1113604e`),
				regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",osd="osd.3"} 1.1113532e`),
				regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",osd="osd.0"} 0.365658`),
				regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",osd="osd.1"} 0.363326`),
				regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",osd="osd.2"} 0.329246`),
				regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",osd="osd.3"} 0.329892`),
				regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_variance{cluster="ceph",osd="osd.0"} 1.053676`),
				regexp.MustCompile(`ceph_osd_variance{cluster="ceph",osd="osd.1"} 1.046957`),
				regexp.MustCompile(`ceph_osd_variance{cluster="ceph",osd="osd.2"} 0.948753`),
				regexp.MustCompile(`ceph_osd_variance{cluster="ceph",osd="osd.3"} 0.950614`),
				regexp.MustCompile(`ceph_osd_variance{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",osd="osd.0"} 283`),
				regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",osd="osd.1"} 279`),
				regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",osd="osd.2"} 162`),
				regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",osd="osd.3"} 164`),
				regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_total_bytes{cluster="ceph"} 4.4601264e`),
				regexp.MustCompile(`ceph_osd_total_used_bytes{cluster="ceph"} 1.5478e`),
				regexp.MustCompile(`ceph_osd_total_avail_bytes{cluster="ceph"} 4.4446484e`),
				regexp.MustCompile(`ceph_osd_average_utilization{cluster="ceph"} 0.347031`),
			},
		},
		{
			input: `
{
    "osd_perf_infos": [
        {
            "id": 4,
            "perf_stats": {
                "commit_latency_ms": 0,
                "apply_latency_ms": 0
            }
        },
        {
            "id": 3,
            "perf_stats": {
                "commit_latency_ms": 1,
                "apply_latency_ms": 64
            }
        },
        {
            "id": 2,
            "perf_stats": {
                "commit_latency_ms": 2,
                "apply_latency_ms": 79
            }
        },
        {
            "id": 1,
            "perf_stats": {
                "commit_latency_ms": 2,
                "apply_latency_ms": 39
            }
        },
        {
            "id": 0,
            "perf_stats": {
                "commit_latency_ms": 2,
                "apply_latency_ms": 31
            }
        }
    ]
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_perf_commit_latency_seconds{cluster="ceph",osd="osd.0"} 0.002`),
				regexp.MustCompile(`ceph_osd_perf_commit_latency_seconds{cluster="ceph",osd="osd.1"} 0.002`),
				regexp.MustCompile(`ceph_osd_perf_commit_latency_seconds{cluster="ceph",osd="osd.2"} 0.002`),
				regexp.MustCompile(`ceph_osd_perf_commit_latency_seconds{cluster="ceph",osd="osd.3"} 0.001`),
				regexp.MustCompile(`ceph_osd_perf_commit_latency_seconds{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_perf_apply_latency_seconds{cluster="ceph",osd="osd.0"} 0.031`),
				regexp.MustCompile(`ceph_osd_perf_apply_latency_seconds{cluster="ceph",osd="osd.1"} 0.039`),
				regexp.MustCompile(`ceph_osd_perf_apply_latency_seconds{cluster="ceph",osd="osd.2"} 0.079`),
				regexp.MustCompile(`ceph_osd_perf_apply_latency_seconds{cluster="ceph",osd="osd.3"} 0.064`),
				regexp.MustCompile(`ceph_osd_perf_apply_latency_seconds{cluster="ceph",osd="osd.4"} 0`),
			},
		},
		{
			input: `
{
    "osds": [
        {
            "osd": 0,
            "uuid": "135b53c3",
            "up": 1,
            "in": 1
        },
        {
            "osd": 1,
            "uuid": "370a33f2",
            "up": 1,
            "in": 1
        },
        {
            "osd": 2,
            "uuid": "ca9ab3de",
            "up": 1,
            "in": 1
        },
        {
            "osd": 3,
            "uuid": "bef98b10",
            "up": 1,
            "in": 1
        },
        {
            "osd": 4,
            "uuid": "5936c9e8",
            "up": 0,
            "in": 0
        }
    ]
}
`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_in{cluster="ceph",osd="osd.0"} 1`),
				regexp.MustCompile(`ceph_osd_in{cluster="ceph",osd="osd.1"} 1`),
				regexp.MustCompile(`ceph_osd_in{cluster="ceph",osd="osd.2"} 1`),
				regexp.MustCompile(`ceph_osd_in{cluster="ceph",osd="osd.3"} 1`),
				regexp.MustCompile(`ceph_osd_in{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_up{cluster="ceph",osd="osd.0"} 1`),
				regexp.MustCompile(`ceph_osd_up{cluster="ceph",osd="osd.1"} 1`),
				regexp.MustCompile(`ceph_osd_up{cluster="ceph",osd="osd.2"} 1`),
				regexp.MustCompile(`ceph_osd_up{cluster="ceph",osd="osd.3"} 1`),
				regexp.MustCompile(`ceph_osd_up{cluster="ceph",osd="osd.4"} 0`),
			},
		},
	} {
		func() {
			collector := NewOSDCollector(NewNoopConn(tt.input), "ceph")
			if err := prometheus.Register(collector); err != nil {
				t.Fatalf("collector failed to register: %s", err)
			}
			defer prometheus.Unregister(collector)

			server := httptest.NewServer(prometheus.Handler())
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("unexpected failed response from prometheus: %s", err)
			}
			defer resp.Body.Close()

			buf, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed reading server response: %s", err)
			}

			for _, re := range tt.regexes {
				if !re.Match(buf) {
					t.Errorf("failed matching: %q", re)
				}
			}
		}()

	}
}
