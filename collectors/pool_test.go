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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func TestPoolInfoCollector(t *testing.T) {
	for _, tt := range []struct {
		input              string
		reMatch, reUnmatch []*regexp.Regexp
	}{
		{
			input: `
[
	{"pool_name": "rbd", "crush_rule": 1, "size": 6, "min_size": 4, "pg_num": 8192, "pg_placement_num": 8192, "quota_max_bytes": 1024, "quota_max_objects": 2048, "erasure_code_profile": "ec-4-2", "stripe_width": 4096},
	{"pool_name": "rbd", "crush_rule": 0, "size": 3, "min_size": 2, "pg_num": 16384, "pg_placement_num": 16384, "quota_max_bytes": 512, "quota_max_objects": 1024, "erasure_code_profile": "replicated-ruleset", "stripe_width": 4096}
]`,
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
			collector := NewPoolInfoCollector(NewNoopConn(tt.input), "ceph", logrus.New())
			if err := prometheus.Register(collector); err != nil {
				t.Fatalf("collector failed to register: %s", err)
			}
			defer prometheus.Unregister(collector)

			server := httptest.NewServer(promhttp.Handler())
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

			for _, re := range tt.reMatch {
				if !re.Match(buf) {
					t.Errorf("failed matching: %q", re)
				}
			}

			for _, re := range tt.reUnmatch {
				if re.Match(buf) {
					t.Errorf("should not have matched: %q", re)
				}
			}
		}()
	}
}
