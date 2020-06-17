package collectors

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	testOSDTreeOutput = `
{
    "stray": [],
    "nodes": [
        {
            "children": [
                -14,
                -15
            ],
            "type_id": 10,
            "type": "root",
            "name": "default",
            "id": -1
        },
        {
            "children": [
                -2
            ],
            "pool_weights": {},
            "type_id": 3,
            "type": "rack",
            "name": "A8R1",
            "id": -14
        },
        {
            "children": [
                -3
            ],
            "pool_weights": {},
            "type_id": 3,
            "type": "rack",
            "name": "A8R2",
            "id": -15
        },
        {
            "children": [
                45,
                44,
                23,
                22,
                21,
                20,
                13,
                12,
                11,
                10,
                4,
                3,
                2,
                1,
                0
            ],
            "pool_weights": {},
            "type_id": 1,
            "type": "host",
            "name": "prod-data01-block01",
            "id": -2
        },
        {
            "children": [
                525,
                524
            ],
            "pool_weights": {},
            "type_id": 1,
            "type": "host",
            "name": "prod-data02-block01",
            "id": -3
        },
        {
            "primary_affinity": 1,
            "reweight": 1,
            "status": "up",
            "exists": 1,
            "id": 0,
            "device_class": "hdd",
            "name": "osd.0",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.950027,
            "status": "up",
            "exists": 1,
            "id": 1,
            "device_class": "ssd",
            "name": "osd.1",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 1,
            "status": "up",
            "exists": 1,
            "id": 2,
            "device_class": "ssd",
            "name": "osd.2",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 3,
            "device_class": "ssd",
            "name": "osd.3",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 4,
            "device_class": "ssd",
            "name": "osd.4",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 10,
            "device_class": "ssd",
            "name": "osd.10",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 11,
            "device_class": "ssd",
            "name": "osd.11",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 12,
            "device_class": "ssd",
            "name": "osd.12",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 13,
            "device_class": "ssd",
            "name": "osd.13",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 20,
            "device_class": "ssd",
            "name": "osd.20",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 21,
            "device_class": "ssd",
            "name": "osd.21",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 22,
            "device_class": "ssd",
            "name": "osd.22",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 23,
            "device_class": "ssd",
            "name": "osd.23",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 44,
            "device_class": "ssd",
            "name": "osd.44",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "primary_affinity": 1,
            "reweight": 0.980011,
            "status": "up",
            "exists": 1,
            "id": 45,
            "device_class": "ssd",
            "name": "osd.45",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 3.481995,
            "depth": 3,
            "pool_weights": {}
        },
        {
            "id": 524,
            "device_class": "ssd",
            "name": "osd.524",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.000000,
            "depth": 0,
            "exists": 1,
            "status": "destroyed",
            "reweight": 0.000000,
            "primary_affinity": 1.000000
        },
        {
            "id": 525,
            "device_class": "ssd",
            "name": "osd.525",
            "type": "osd",
            "type_id": 0,
            "crush_weight": 0.000000,
            "depth": 0,
            "exists": 1,
            "status": "destroyed",
            "reweight": 0.000000,
            "primary_affinity": 1.000000
        }
    ]
}
`
)

func TestOSDLabelBuilder(t *testing.T) {
	osds, err := buildOSDLabels([]byte(testOSDTreeOutput))
	if err != nil {
		t.Fatal(err)
	}

	osd, ok := osds[0]
	if !ok {
		t.Fatal("did not find expected node in map")
	}

	if osd.Type != "osd" {
		t.Fatal("node is not an osd")
	}

	if osd.Rack != "A8R1" {
		t.Fatal("node is not in expected rack")
	}

	if osd.Host != "prod-data01-block01" {
		t.Fatal("node is not in expected host")
	}

	if osd.Root != "default" {
		t.Fatal("node is not in expected root", osd.Root)
	}

	if osd.DeviceClass != "hdd" {
		t.Fatal("node is not an hdd")
	}
}

func TestOSDCollector(t *testing.T) {
	for _, tt := range []struct {
		cmdOut  map[string]string
		regexes []*regexp.Regexp
	}{
		{
			cmdOut: map[string]string{
				"ceph osd df": `
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
			},
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
			cmdOut: map[string]string{
				"ceph osd perf": `
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
			},
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
			cmdOut: map[string]string{
				"ceph osd dump": `
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
      "in": 1,
      "state": [
        "nearfull",
        "exists",
        "up"
      ]
    },
    {
      "osd": 3,
      "uuid": "bef98b10",
      "up": 1,
      "in": 1,
      "state": [
        "full",
        "backfillfull",
        "exists",
        "up"
      ]
    },
    {
      "osd": 4,
      "uuid": "5936c9e8",
      "up": 0,
      "in": 0,
      "state": [
        "backfillfull",
        "exists",
        "up"
      ]
    }
  ]
}`,
			},
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
				regexp.MustCompile(`ceph_osd_full{cluster="ceph",osd="osd.0"} 0`),
				regexp.MustCompile(`ceph_osd_full{cluster="ceph",osd="osd.1"} 0`),
				regexp.MustCompile(`ceph_osd_full{cluster="ceph",osd="osd.2"} 0`),
				regexp.MustCompile(`ceph_osd_full{cluster="ceph",osd="osd.3"} 1`),
				regexp.MustCompile(`ceph_osd_full{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",osd="osd.0"} 0`),
				regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",osd="osd.1"} 0`),
				regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",osd="osd.2"} 1`),
				regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",osd="osd.3"} 0`),
				regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",osd="osd.4"} 0`),
				regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",osd="osd.0"} 0`),
				regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",osd="osd.1"} 0`),
				regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",osd="osd.2"} 0`),
				regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",osd="osd.3"} 1`),
				regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",osd="osd.4"} 1`),
			},
		},
		{
			cmdOut: map[string]string{
				"ceph pg dump pgs_brief": `
[
  {
    "acting": [
      1,
      2,
      3,
      4
    ],
    "acting_primary": 1,
    "pgid": "81.1fff",
    "state": "active+clean"
  },
  {
    "acting": [
      10,
      11,
      12,
      13
    ],
    "acting_primary": 10,
    "pgid": "82.1fff",
    "state": "active+clean+scrubbing"
  },
  {
    "acting": [
      20,
      21,
      22,
      23
    ],
    "acting_primary": 20,
    "pgid": "83.1fff",
    "state": "active+clean+scrubbing+deep"
  }
]`,
			},
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.10"} 1`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.11"} 1`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.12"} 1`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.13"} 1`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.20"} 2`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.21"} 2`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.22"} 2`),
				regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",osd="osd.23"} 2`),
			},
		},
		{
			cmdOut: map[string]string{
				"ceph osd tree down": `
{
  "nodes": [],
  "stray": [
    {
      "id": 524,
      "name": "osd.524",
      "type": "osd",
      "type_id": 0,
      "crush_weight": 0.000000,
      "depth": 0,
      "exists": 1,
      "status": "destroyed",
      "reweight": 0.000000,
      "primary_affinity": 1.000000
    }
  ]
}`,
			},
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",osd="osd.524",status="destroyed"} 1`),
			},
		},
		{
			cmdOut: map[string]string{
				"ceph osd tree down": `
{
  "nodes": [],
  "stray": [
    {
      "id": 524,
      "name": "osd.524",
      "type": "osd",
      "type_id": 0,
      "crush_weight": 0.000000,
      "depth": 0,
      "exists": 1,
      "status": "down",
      "reweight": 0.000000,
      "primary_affinity": 1.000000
    }
  ]
}`,
			},
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",osd="osd.524",status="down"} 1`),
			},
		},
		{
			cmdOut: map[string]string{
				"ceph osd tree down": `
{
  "nodes": [
    {
      "id": -18,
      "name": "data",
      "type": "root",
      "type_id": 10,
      "children": [
        -20
      ]
    },
    {
      "id": -20,
      "name": "R1-data",
      "type": "rack",
      "type_id": 3,
      "pool_weights": {},
      "children": [
        -8
      ]
    },
    {
      "id": -8,
      "name": "test-data03-object01",
      "type": "host",
      "type_id": 1,
      "pool_weights": {},
      "children": [
        97
      ]
    },
    {
      "id": 524,
      "device_class": "hdd",
      "name": "osd.524",
      "type": "osd",
      "type_id": 0,
      "crush_weight": 7.265991,
      "depth": 3,
      "pool_weights": {},
      "exists": 1,
      "status": "destroyed",
      "reweight": 0.000000,
      "primary_affinity": 1.000000
    }
  ],
  "stray": []
}`,
			},
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",osd="osd.524",status="destroyed"} 1`),
			},
			regExp: []*regexp.Regexp{},
		},
		{
			cmdOut: map[string]string{
				"ceph osd tree down": `
{
  "nodes": [
    {
      "id": -18,
      "name": "data",
      "type": "root",
      "type_id": 10,
      "children": [
        -20
      ]
    },
    {
      "id": -20,
      "name": "R1-data",
      "type": "rack",
      "type_id": 3,
      "pool_weights": {},
      "children": [
        -8
      ]
    },
    {
      "id": -8,
      "name": "test-data03-object01",
      "type": "host",
      "type_id": 1,
      "pool_weights": {},
      "children": [
        97
      ]
    },
    {
      "id": 524,
      "device_class": "hdd",
      "name": "osd.524",
      "type": "osd",
      "type_id": 0,
      "crush_weight": 7.265991,
      "depth": 3,
      "pool_weights": {},
      "exists": 1,
      "status": "destroyed",
      "reweight": 0.000000,
      "primary_affinity": 1.000000
    }
  ],
  "stray": [
    {
      "id": 525,
      "name": "osd.525",
      "type": "osd",
      "type_id": 0,
      "crush_weight": 0.000000,
      "depth": 0,
      "exists": 1,
      "status": "down",
      "reweight": 0.000000,
      "primary_affinity": 1.000000
    }
  ]
}`,
			},
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",osd="osd.524",status="destroyed"} 1`),
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",osd="osd.525",status="down"} 1`),
			},
		},
		{
			cmdOut: map[string]string{
				"ceph osd tree down": `
{
  "nodes": []}}
}`,
			},
			regexes: []*regexp.Regexp{},
		},
		{
			cmdOut: map[string]string{
				"ceph pg dump pgs_brief": `
[
  {
    "acting": [
      1,
      2,
      3,
      4
    ],
    "acting_primary": 1,
    "pgid": "81.1fff",
    "state": "active+clean"
  },
  {
    "acting": [
      10,
      11,
      12,
      13
    ],
    "acting_primary": 10,
    "pgid": "82.1fff",
    "state": "active+clean+scrubbing"
  },
  {
    "acting": [
      20,
      21,
      22,
      23
    ],
    "acting_primary": 20,
    "pgid": "83.1fff",
    "state": "active+clean+scrubbing+deep"
  },
  {
    "acting": [
      30,
      31,
      32,
      33
    ],
    "acting_primary": 30,
    "pgid": "84.1fff",
    "state": "active+recovering+degraded"
  }
]`,
				"ceph tell 84.1fff query": `
{
  "state": "active+recovering+degraded",
  "info": {
    "stats": {
      "stat_sum": {
        "num_objects_recovered": 123
      }
    }
  }
}`,
			},
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`ceph_pg_objects_recovered_total{cluster="ceph",pgid="84.1fff"} 0`),
			},
		},
	} {
		func() {
			collector := NewOSDCollector(NewNoopConnWithCmdOut(tt.cmdOut), "ceph")
			if err := prometheus.Register(collector); err != nil {
				t.Fatalf("collector failed to register: %s", err)
			}
			defer prometheus.Unregister(collector)

			server := httptest.NewServer(prometheus.Handler())
			defer server.Close()

			var buf []byte

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("unexpected failed response from prometheus: %s", err)
			}
			defer resp.Body.Close()

			buf, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed reading server response: %s", err)
			}

			for _, re := range tt.regexes {
				if !re.Match(buf) {
					t.Fatalf("failed matching: %q %s", re, buf)
				}
			}
		}()

	}
}
