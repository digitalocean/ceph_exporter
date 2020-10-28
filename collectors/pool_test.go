//   Copyright 2019 DigitalOcean
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
	"encoding/json"
	"fmt"
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

func TestPoolInfoCollector(t *testing.T) {
	for _, tt := range []struct {
		reMatch, reUnmatch []*regexp.Regexp
	}{
		{
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_size{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 6`),
				regexp.MustCompile(`pool_min_size{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 4`),
				regexp.MustCompile(`pool_pg_num{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 8192`),
				regexp.MustCompile(`pool_pgp_num{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 8192`),
				regexp.MustCompile(`pool_quota_max_bytes{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 1024`),
				regexp.MustCompile(`pool_quota_max_objects{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 2048`),
				regexp.MustCompile(`pool_stripe_width{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 4096`),
				regexp.MustCompile(`pool_expansion_factor{cluster="ceph",pool="rbd",profile="ec-4-2",root="non-default-root"} 1.5`),

				regexp.MustCompile(`pool_size{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 3`),
				regexp.MustCompile(`pool_min_size{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 2`),
				regexp.MustCompile(`pool_pg_num{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 16384`),
				regexp.MustCompile(`pool_pgp_num{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 16384`),
				regexp.MustCompile(`pool_quota_max_bytes{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 512`),
				regexp.MustCompile(`pool_quota_max_objects{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 1024`),
				regexp.MustCompile(`pool_stripe_width{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 4096`),
				regexp.MustCompile(`pool_expansion_factor{cluster="ceph",pool="rbd",profile="replicated-ruleset",root="default"} 3`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
	} {
		func() {
			conn := &mocks.Conn{}
			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd pool ls",
					"detail": "detail",
					"format": "json",
				})
			})).Return([]byte(`
[
	{"pool_name": "rbd", "crush_rule": 1, "size": 6, "min_size": 4, "pg_num": 8192, "pg_placement_num": 8192, "quota_max_bytes": 1024, "quota_max_objects": 2048, "erasure_code_profile": "ec-4-2", "stripe_width": 4096},
	{"pool_name": "rbd", "crush_rule": 0, "size": 3, "min_size": 2, "pg_num": 16384, "pg_placement_num": 16384, "quota_max_bytes": 512, "quota_max_objects": 1024, "erasure_code_profile": "replicated-ruleset", "stripe_width": 4096}
]`,
			), "", nil)

			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd crush rule dump",
					"format": "json",
				})
			})).Return([]byte(`
[
  {
	"rule_id": 0,
	"rule_name": "replicated_rule",
	"ruleset": 0,
	"type": 1,
	"min_size": 1,
	"max_size": 10,
	"steps": [
	  {
		"num": 5,
		"op": "set_chooseleaf_tries"
	  },
	  {
		"op": "take",
		"item": -1,
		"item_name": "default"
	  },
	  {
		"op": "chooseleaf_firstn",
		"num": 0,
		"type": "host"
	  },
	  {
		"op": "emit"
	  }
	]
  },
  {
	"rule_id": 1,
	"rule_name": "another-rule",
	"ruleset": 1,
	"type": 1,
	"min_size": 1,
	"max_size": 10,
	"steps": [
	  {
		"op": "take",
		"item": -53,
		"item_name": "non-default-root"
	  },
	  {
		"op": "chooseleaf_firstn",
		"num": 0,
		"type": "rack"
	  },
	  {
		"op": "emit"
	  }
	]
  }
]`,
			), "", nil)

			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return cmp.Equal(v, map[string]interface{}{
					"prefix": "osd erasure-code-profile get",
					"name":   "ec-4-2",
					"format": "json",
				})
			})).Return([]byte(`
{
	"crush-device-class": "",
	"crush-failure-domain": "host",
	"crush-root": "objectdata",
	"jerasure-per-chunk-alignment": "false",
	"k": "4",
	"m": "2",
	"plugin": "jerasure",
	"technique": "reed_sol_van",
	"w": "8"
}`,
			), "", nil)

			conn.On("MonCommand", mock.MatchedBy(func(in interface{}) bool {
				v := map[string]interface{}{}

				err := json.Unmarshal(in.([]byte), &v)
				require.NoError(t, err)

				return !cmp.Equal(v, map[string]interface{}{
					"prefix": "osd erasure-code-profile get",
					"name":   "ec-4-2",
					"format": "json",
				})
			})).Return([]byte(""), "", fmt.Errorf("unknown erasure code profile"))

			collector := NewPoolInfoCollector(conn, "ceph", logrus.New())

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
			for _, re := range tt.reUnmatch {
				require.False(t, re.Match(buf))
			}
		}()
	}
}
