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
	"encoding/json"
	"io/ioutil"
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
		input   []byte
		reMatch []*regexp.Regexp
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
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rbd_mirror_pool_status{cluster="ceph"} 2`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_daemon_status{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rbd_mirror_pool_image_status{cluster="ceph"} 2`),
			},
		},
	} {
		func() {
			collector := NewRbdMirrorStatusCollector(&Exporter{Cluster: "ceph", Version: Pacific, Logger: logrus.New()})

			var rbdStatus rbdMirrorPoolStatus
			setStatus(tt.input)
			_ = json.Unmarshal([]byte(tt.input), &rbdStatus)

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
