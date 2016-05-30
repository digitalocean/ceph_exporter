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

func TestDfCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		regexes []*regexp.Regexp
	}{
		{
			`
{
    "stats": {
        "total_bytes": 255205081620480,
        "total_used_bytes": 17850383970304,
        "total_avail_bytes": 237354697650176
    },
    "pools": [
        {
            "name": "rbd",
            "id": 1,
            "stats": {
                "kb_used": 4715700422,
                "bytes_used": 4828877231187,
                "max_avail": 65868981633781,
                "objects": 1360283
            }
        },
        {
            "name": "ssd",
            "id": 2,
            "stats": {
                "kb_used": 362463233,
                "bytes_used": 371162350522,
                "max_avail": 4621426774653,
                "objects": 89647
            }
        },
        {
            "name": "bench",
            "id": 3,
            "stats": {
                "kb_used": 0,
                "bytes_used": 0,
                "max_avail": 4621426774653,
                "objects": 0
            }
        }
    ]
}
`,
			[]*regexp.Regexp{
				regexp.MustCompile(`ceph_df_pool_bytes_used{pool="bench"} 0`),
				regexp.MustCompile(`ceph_df_pool_bytes_used{pool="rbd"} 4.828877231187e\+12`),
				regexp.MustCompile(`ceph_df_pool_bytes_used{pool="ssd"} 3.71162350522e\+11`),
				regexp.MustCompile(`ceph_df_pool_bytes_available{pool="bench"} 4.621426774653e\+12`),
				regexp.MustCompile(`ceph_df_pool_bytes_available{pool="rbd"} 6.5868981633781e\+13`),
				regexp.MustCompile(`ceph_df_pool_bytes_available{pool="ssd"} 4.621426774653e\+12`),
			},
		},
	} {
		func() {
			collector := NewDfCollector(NewNoopConn(tt.input))
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
