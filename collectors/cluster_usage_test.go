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
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClusterUsage(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	for _, tt := range []struct {
		input              string
		reMatch, reUnmatch []*regexp.Regexp
	}{
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_avail_bytes": 4,
		"total_objects": 1
	}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes 4`),
				regexp.MustCompile(`ceph_cluster_objects 1`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_used_bytes": 6,
		"total_avail_bytes": 4,
		"total_objects": 1
	}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes 0`),
				regexp.MustCompile(`ceph_cluster_used_bytes 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes 4`),
				regexp.MustCompile(`ceph_cluster_objects 1`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_avail_bytes": 4,
		"total_objects": 1
	}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes 0`),
				regexp.MustCompile(`ceph_cluster_available_bytes 4`),
				regexp.MustCompile(`ceph_cluster_objects 1`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_objects": 1
	}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes 0`),
				regexp.MustCompile(`ceph_cluster_objects 1`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_avail_bytes": 4
	}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes 4`),
				regexp.MustCompile(`ceph_cluster_objects 0`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {{{
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_avail_bytes": 4,
		"total_objects": 1
	}
}`,
			reMatch: []*regexp.Regexp{},
			reUnmatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes`),
				regexp.MustCompile(`ceph_cluster_used_bytes`),
				regexp.MustCompile(`ceph_cluster_available_bytes`),
				regexp.MustCompile(`ceph_cluster_objects`),
			},
		},
	} {
		func() {
			collector := NewClusterUsageCollector(NewNoopConn(tt.input))
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
