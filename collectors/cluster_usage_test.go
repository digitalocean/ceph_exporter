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

	"github.com/prometheus/client_golang/prometheus"
)

func TestClusterUsage(t *testing.T) {
	var (
		expected = `
{
	"stats": {
		"total_bytes": 10,
		"total_used_bytes": 6,
		"total_avail_bytes": 4,
		"total_objects": 1
	}
}`
	)

	collector := NewClusterUsageCollector(NewNoopConn(expected))
	if err := prometheus.Register(collector); err != nil {
		t.Fatalf("collector failed to register: %s", err)
	}

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

	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`ceph_cluster_bytes_total 10`),
		regexp.MustCompile(`ceph_cluster_used_bytes 6`),
		regexp.MustCompile(`ceph_cluster_available_bytes 4`),
		regexp.MustCompile(`ceph_objects_total 1`),
	} {
		if !re.Match(buf) {
			t.Errorf("failed matching: %q", re)
		}
	}
}
