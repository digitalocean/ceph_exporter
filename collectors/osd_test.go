package collectors

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/digitalocean/ceph_exporter/mocks"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	osd, ok := osds[0]
	require.Truef(t, ok, "expect to find node in map")
	require.Equalf(t, "osd", osd.Type, "expect to be an OSD")
	require.Equalf(t, "A8R1", osd.Rack, "expect to be in the rack")
	require.Equalf(t, "prod-data01-block01", osd.Host, "expect to be in the host")
	require.Equalf(t, "default", osd.Root, "expect root to be default")
	require.Equalf(t, "hdd", osd.DeviceClass, "expect to be an HDD")
}

func TestOSDCollector(t *testing.T) {
	reMatch := []*regexp.Regexp{
		regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 0.010391`),
		regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 0.010391`),
		regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 0.010391`),
		regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 0.010391`),
		regexp.MustCompile(`ceph_osd_crush_weight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0.010391`),
		regexp.MustCompile(`ceph_osd_depth{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_depth{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_depth{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_depth{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_depth{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_reweight{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 1.1417923584e`),
		regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 1.1417923584e`),
		regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 1.1417923584e`),
		regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1.1417923584e`),
		regexp.MustCompile(`ceph_osd_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 4.1750528e`),
		regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 4.1484288e`),
		regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 3.7593088e`),
		regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 3.7666816e`),
		regexp.MustCompile(`ceph_osd_used_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 1.1376173056e`),
		regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 1.1376439296e`),
		regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 1.1380330496e`),
		regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1.1380256768e`),
		regexp.MustCompile(`ceph_osd_avail_bytes{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 0.365658`),
		regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 0.363326`),
		regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 0.329246`),
		regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 0.329892`),
		regexp.MustCompile(`ceph_osd_utilization{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_variance{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 1.053676`),
		regexp.MustCompile(`ceph_osd_variance{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 1.046957`),
		regexp.MustCompile(`ceph_osd_variance{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 0.948753`),
		regexp.MustCompile(`ceph_osd_variance{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 0.950614`),
		regexp.MustCompile(`ceph_osd_variance{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 283`),
		regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 279`),
		regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 162`),
		regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 164`),
		regexp.MustCompile(`ceph_osd_pgs{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_pg_upmap_items_total{cluster="ceph"} 2`),
		regexp.MustCompile(`ceph_osd_total_bytes{cluster="ceph"} 4.5671694336e`),
		regexp.MustCompile(`ceph_osd_total_used_bytes{cluster="ceph"} 1.5849472e`),
		regexp.MustCompile(`ceph_osd_total_avail_bytes{cluster="ceph"} 4.5513199616e`),
		regexp.MustCompile(`ceph_osd_average_utilization{cluster="ceph"} 0.347031`),
		regexp.MustCompile(`ceph_osd_near_full_ratio{cluster="ceph"} 0.7`),
		regexp.MustCompile(`ceph_osd_backfill_full_ratio{cluster="ceph"} 0.8`),
		regexp.MustCompile(`ceph_osd_full_ratio{cluster="ceph"} 0.9`),
		regexp.MustCompile(`ceph_osd_in{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_in{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_in{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_in{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_in{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_up{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_up{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_up{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_up{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_up{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_full{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_near_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",device_class="hdd",host="prod-data01-block01",osd="osd.0",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.1",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.2",rack="A8R1",root="default"} 0`),
		regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.3",rack="A8R1",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_backfill_full{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.4",rack="A8R1",root="default"} 1`),

		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.10",rack="default",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.11",rack="default",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.12",rack="default",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.13",rack="default",root="default"} 1`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.20",rack="default",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.21",rack="default",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.22",rack="default",root="default"} 2`),
		regexp.MustCompile(`ceph_osd_scrub_state{cluster="ceph",device_class="ssd",host="prod-data01-block01",osd="osd.23",rack="default",root="default"} 2`),
	}

	for _, tt := range []struct {
		test    string
		reMatch []*regexp.Regexp
	}{
		{
			test: "1",
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",device_class="ssd",host="prod-data02-block01",osd="osd.524",rack="default",root="A8R2",status="destroyed"} 1`),
			},
		},
		{
			test: "2",
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",device_class="ssd",host="prod-data02-block01",osd="osd.524",rack="default",root="A8R2",status="down"} 1`),
			},
		},
		{
			test: "3",
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",device_class="ssd",host="prod-data02-block01",osd="osd.524",rack="default",root="A8R2",status="destroyed"} 1`),
			},
		},
		{
			test: "4",
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",device_class="ssd",host="prod-data02-block01",osd="osd.524",rack="default",root="A8R2",status="destroyed"} 1`),
				regexp.MustCompile(`ceph_osd_down{cluster="ceph",device_class="ssd",host="prod-data02-block01",osd="osd.525",rack="default",root="A8R2",status="down"} 1`),
			},
		},
		{
			test:    "5",
			reMatch: []*regexp.Regexp{},
		},
	} {
		func() {
			conn := &mocks.Conn{}

			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd tree",
					"format": "json",
				})
			})).Return([]byte(testOSDTreeOutput), "", nil)

			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd tree",
					"states": []interface{}{"down"},
					"format": "json",
				})
			})).Return([]byte(map[string]string{
				"1": `
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

				"2": `
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
				"3": `
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
				"4": `
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
				"5": `
{
  "nodes": []}}
}`,
			}[tt.test]), "", nil)

			conn.On("MgrCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				uv, ok := in.([][]byte)
				require.True(t, ok)
				require.Len(t, uv, 1)

				err := json.Unmarshal(uv[0], &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix":       "pg dump",
					"dumpcontents": []interface{}{"pgs_brief"},
					"format":       "json",
				})
			})).Return([]byte(`
{
	"pg_ready": true,
	"pg_stats": [
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
	]
}`), "", nil)

			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd dump",
					"format": "json",
				})
			})).Return([]byte(`
{
	"full_ratio": 0.9,
	"backfillfull_ratio": 0.8,
	"nearfull_ratio": 0.7,
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
	],
	"pg_upmap_items": [
		{
			"pgid": "1.8f",
			"mappings": [
				{
					"from": 37,
					"to": 36
				}
			]
		},
		{
			"pgid": "1.90",
			"mappings": [
				{
					"from": 37,
					"to": 36
				},
				{
					"from": 31,
					"to": 30
				}
			]
		}
	]
}`), "", nil)

			conn.On("MgrCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				uv, ok := in.([][]byte)
				require.True(t, ok)
				require.Len(t, uv, 1)

				err := json.Unmarshal(uv[0], &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd df",
					"format": "json",
				})
			})).Return([]byte(`
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
}`), "", nil)

			conn.On("MgrCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				uv, ok := in.([][]byte)
				require.True(t, ok)
				require.Len(t, uv, 1)

				err := json.Unmarshal(uv[0], &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd perf",
					"format": "json",
				})
			})).Return([]byte(`
{
    "osdstats": {
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
    }
}`), "", nil)

			collector := NewOSDCollector(conn, "ceph", logrus.New())
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

			for _, re := range append(reMatch, tt.reMatch...) {
				require.True(t, re.Match(buf))
			}
		}()
	}
}
