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

func TestCrashesCollector(t *testing.T) {

	for _, tt := range []struct {
		name    string
		input   string
		reMatch []*regexp.Regexp
	}{
		{
			name: "single new crash",
			input: `
ID                                                               ENTITY  NEW 
2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9 osd.0    *  
			`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.0",status="new"} 1`),
			},
		},
		{
			name: "single archived crash",
			input: `
ID                                                               ENTITY  NEW 
2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9 osd.0       
			`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.0",status="archived"} 1`),
			},
		},
		{
			name: "two new crashes same entity",
			input: `
ID                                                               ENTITY  NEW 
2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9 osd.0    *  
2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a osd.0    *  
`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.0",status="new"} 2`),
			},
		},
		{
			name: "mix of crashes same entity",
			input: `
ID                                                               ENTITY  NEW 
2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9 osd.0       
2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a osd.0    *  
`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.0",status="new"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.0",status="archived"} 1`),
			},
		},
		{
			name: "mix of crashes different entities",
			input: `
ID                                                               ENTITY          NEW 
2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9 mgr.mgr-node-01  *  
2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a client.admin     *  
`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="mgr.mgr-node-01",status="new"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="client.admin",status="new"} 1`),
			},
		},
		{
			// At least code shouldn't panic
			name:    "no crashes",
			input:   ``,
			reMatch: []*regexp.Regexp{},
		},
	} {
		t.Run(
			tt.name,
			func(t *testing.T) {
				conn := &MockConn{}
				conn.On("MonCommand", mock.Anything).Return(
					[]byte(tt.input), "", nil,
				)

				collector := NewCrashesCollector(&Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New(), Version: Pacific})
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
					if !re.Match(buf) {
						t.Errorf("expected %s to match\n", re.String())
					}
				}
			},
		)
	}
}
