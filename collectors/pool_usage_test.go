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

func TestPoolUsageCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		regexes []*regexp.Regexp
	}{
		{
			`
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "objects": 5, "rd": 4, "wr": 6}}
]}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 20`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 5`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 4`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 6`),
			},
		},
	} {
		func() {
			collector := NewPoolUsageCollector(NewNoopConn(tt.input))
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
