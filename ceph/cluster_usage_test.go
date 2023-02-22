//   Copyright 2022 DigitalOcean
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
	"io/ioutil"
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

func TestClusterUsage(t *testing.T) {
	for _, tt := range []struct {
		input              string
		version            string
		reMatch, reUnmatch []*regexp.Regexp
	}{
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_avail_bytes": 4
	}
}`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes{cluster="ceph"} 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes{cluster="ceph"} 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes{cluster="ceph"} 4`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_used_bytes": 6,
		"total_avail_bytes": 4
	}
}`,
			version: `{"version":"ceph version 16.2.11-98-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_cluster_used_bytes{cluster="ceph"} 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes{cluster="ceph"} 4`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_avail_bytes": 4
	}
}`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes{cluster="ceph"} 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_cluster_available_bytes{cluster="ceph"} 4`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {
		"total_bytes": 10,
		"total_used_bytes": 6
	}
}`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes{cluster="ceph"} 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes{cluster="ceph"} 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes{cluster="ceph"} 0`),
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
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes{cluster="ceph"} 10`),
				regexp.MustCompile(`ceph_cluster_used_bytes{cluster="ceph"} 6`),
				regexp.MustCompile(`ceph_cluster_available_bytes{cluster="ceph"} 4`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{
	"stats": {{{
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_avail_bytes": 4
	}
}`,
			version: `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			reMatch: []*regexp.Regexp{},
			reUnmatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_cluster_capacity_bytes{cluster="ceph"}`),
				regexp.MustCompile(`ceph_cluster_used_bytes{cluster="ceph"}`),
				regexp.MustCompile(`ceph_cluster_available_bytes{cluster="ceph"}`),
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
				"clusterUsage": NewClusterUsageCollector(e),
			}
			err := prometheus.Register(e)
			require.NoError(t, err)
			defer prometheus.Unregister(e)

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
