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
	"github.com/stretchr/testify/require"
)

func setStatus(b []byte) {
	rbdMirrorStatus = func(string, string) ([]byte, error) {
		return b, nil
	}
}

func TestRbdMirrorStatusCollector(t *testing.T) {

	for _, tt := range []struct {
		input    []byte
		version  string
		versions string
		reMatch  []*regexp.Regexp
	}{
		{
			input: []byte(`
			{
				"summary": {
				  "health": "WARNING",
				  "daemon_health": "OK",
				  "image_health": "WARNING",
				  "states": {
					"unknown": 1
				  }
				}
			  }`),
			version:  `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			versions: `{"rbd-mirror":{"ceph version 16.2.11-98-g1984a8c (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)":3}}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rbd_mirror_pool_status{cluster="ceph"} 1`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_image_status{cluster="ceph"} 1`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_daemon_status{cluster="ceph"} 0`),
			},
		},
		{
			input: []byte(`
			{
				"summary": {
				  "health": "WARNING",
				  "daemon_health": "WARNING",
				  "image_health": "OK",
				  "states": {}
				}
			  }`),
			version:  `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			versions: `{"rbd-mirror":{"ceph version 16.2.11-98-g1984a8c (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)":3}}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rbd_mirror_pool_status{cluster="ceph"} 1`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_daemon_status{cluster="ceph"} 1`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_image_status{cluster="ceph"} 0`),
			},
		},
		{
			input: []byte(`
			{
				"summary": {
				  "health": "OK",
				  "daemon_health": "OK",
				  "image_health": "OK",
				  "states": {}
				}
			  }`),
			version:  `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			versions: `{"rbd-mirror":{"ceph version 16.2.11-98-g1984a8c (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)":3}}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rbd_mirror_pool_status{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_daemon_status{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_image_status{cluster="ceph"} 0`),
			},
		},
		{
			input: []byte(`
			{
				"summary": {
				  "health": "ERROR",
				  "daemon_health": "OK",
				  "image_health": "ERROR",
				  "states": {}
				}
			  }`),
			version:  `{"version":"ceph version 16.2.11-22-wasd (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)"}`,
			versions: `{"rbd-mirror":{"ceph version 16.2.11-98-g1984a8c (1984a8c33225d70559cdf27dbab81e3ce153f6ac) pacific (stable)":3}}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rbd_mirror_pool_status{cluster="ceph"} 2`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_daemon_status{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_image_status{cluster="ceph"} 2`),
			},
		},
		{
			input: []byte(`
			{
				"summary": {
				  "health": "OK",
				  "daemon_health": "OK",
				  "image_health": "ERROR"
				}
			  }`),
			version:  `{"version":"ceph version 14.2.9-12-zasd (1337) pacific (stable)"}`,
			versions: `{"rbd-mirror":{"ceph version 14.2.9-12-zasd (1337) pacific (stable)":3}}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rbd_mirror_pool_status{cluster="ceph"} 0`),
			},
		},
	} {
		func() {
			conn := setupVersionMocks(tt.version, tt.versions)

			e := &Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New()}
			// We do not create the rbdCollector since it will
			// be automatically initiated from the output of `ceph versions`
			// if the rbd-mirror key is present
			e.cc = map[string]versionedCollector{}

			err := prometheus.Register(e)
			require.NoError(t, err)
			defer prometheus.Unregister(e)

			setStatus(tt.input)

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
