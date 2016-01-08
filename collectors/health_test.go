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

func TestClusterHealthCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		regexes []*regexp.Regexp
	}{
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 0,
            "num_up_osds": 0,
            "num_in_osds": 0,
            "num_remapped_pgs": 0
        }
    },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "5 pgs degraded"}]}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`degraded_pgs 5`),
			},
		},
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 0,
            "num_up_osds": 0,
            "num_in_osds": 0,
            "num_remapped_pgs": 0
        }
    },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "6 pgs stuck unclean"}]}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`unclean_pgs 6`),
			},
		},
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 0,
            "num_up_osds": 0,
            "num_in_osds": 0,
            "num_remapped_pgs": 0
        }
    },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "7 pgs undersized"}]}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`undersized_pgs 7`),
			},
		},
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 0,
            "num_up_osds": 0,
            "num_in_osds": 0,
            "num_remapped_pgs": 0
        }
    },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "8 pgs stale"}]}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`stale_pgs 8`),
			},
		},
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 0,
            "num_up_osds": 0,
            "num_in_osds": 0,
            "num_remapped_pgs": 0
        }
    },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "recovery 10/20 objects degraded"}]}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`degraded_objects 10`),
			},
		},
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 0,
            "num_up_osds": 0,
            "num_in_osds": 0,
            "num_remapped_pgs": 0
        }
    },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "3/20 in osds are down"}]}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`osds_down 3`),
			},
		},
		{
			`
{
    "osdmap": {
        "osdmap": {
            "num_osds": 1200,
            "num_up_osds": 1200,
            "num_in_osds": 1190,
            "num_remapped_pgs": 10
        }
    },
	"health": {"summary": []}
}`,
			[]*regexp.Regexp{
				regexp.MustCompile(`osds 1200`),
				regexp.MustCompile(`osds_up 1200`),
				regexp.MustCompile(`osds_in 1190`),
				regexp.MustCompile(`pgs_remapped 10`),
			},
		},
	} {
		func() {
			collector := NewClusterHealthCollector(NewNoopConn(tt.input))
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
