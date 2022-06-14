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
	"reflect"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCrashesCollector(t *testing.T) {

	const outputCephCrashLs string = `
	ID                                                               ENTITY          NEW 
	2022-01-01_18:57:51.184156Z_02d9b659-69d1-4dd6-8495-ee2345208568 client.admin        
	2022-01-01_19:02:01.401852Z_9100163b-4cd1-479f-b3a8-0dc2d288eaea mgr.mgr-node-01
	2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9 client.admin     *  
	2022-02-03_04:03:38.371403Z_bd756324-27c0-494e-adfb-9f5f6e3db000 osd.3            *  
	2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a osd.3            *  
	`

	t.Run(
		"full test",
		func(t *testing.T) {
			conn := &MockConn{}
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(outputCephCrashLs), "", nil,
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

			reMatches := []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="client.admin",status="new"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="client.admin",status="archived"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="mgr.mgr-node-01",status="new"} 0`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="mgr.mgr-node-01",status="archived"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.3",status="new"} 2`),
				regexp.MustCompile(`crash_reports{cluster="ceph",daemon="osd.3",status="archived"} 0`),
			}

			// t.Log(string(buf))
			for _, re := range reMatches {
				if !re.Match(buf) {
					t.Errorf("expected %s to match\n", re.String())
				}
			}
		},
	)

	t.Run(
		"getCrashLs unit test",
		func(t *testing.T) {
			conn := &MockConn{}
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(outputCephCrashLs), "", nil,
			)

			log := logrus.New()
			log.Level = logrus.DebugLevel
			collector := NewCrashesCollector(&Exporter{Conn: conn, Cluster: "ceph", Logger: log, Version: Pacific})

			expected := []crashEntry{
				{"client.admin", false},
				{"mgr.mgr-node-01", false},
				{"client.admin", true},
				{"osd.3", true},
				{"osd.3", true},
			}
			crashes, _ := collector.getCrashLs()

			if !reflect.DeepEqual(crashes, expected) {
				t.Errorf("incorrect getCrashLs result: expected %v, got %v\n", expected, crashes)
			}
		},
	)

	t.Run(
		"getCrashLs empty crash list unit test",
		func(t *testing.T) {
			conn := &MockConn{}
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(""), "", nil,
			)

			collector := NewCrashesCollector(&Exporter{Conn: conn, Cluster: "ceph", Logger: logrus.New(), Version: Pacific})

			crashes, _ := collector.getCrashLs()
			if len(crashes) != 0 {
				t.Errorf("expected empty result from getCrashLs, got %v\n", crashes)
			}
		},
	)

	t.Run(
		"processCrashLs test",
		func(t *testing.T) {
			collector := NewCrashesCollector(&Exporter{Conn: nil, Cluster: "ceph", Logger: logrus.New(), Version: Pacific})

			newCrash := crashEntry{"daemon", true}
			archivedCrash := crashEntry{"daemon", false}

			// New crash
			crashMap := collector.processCrashLs([]crashEntry{newCrash})
			expected := map[crashEntry]int{newCrash: 1, archivedCrash: 0}
			if !reflect.DeepEqual(crashMap, expected) {
				t.Errorf("incorrect processCrashLs result: expected %v, got %v\n", expected, crashMap)
			}

			// Archived crash
			crashMap = collector.processCrashLs([]crashEntry{archivedCrash})
			expected = map[crashEntry]int{newCrash: 0, archivedCrash: 1}
			if !reflect.DeepEqual(crashMap, expected) {
				t.Errorf("incorrect processCrashLs result: expected %v, got %v\n", expected, crashMap)
			}

			// Crash was memorized, check that we reset count to zero
			crashMap = collector.processCrashLs([]crashEntry{})
			expected = map[crashEntry]int{newCrash: 0, archivedCrash: 0}
			if !reflect.DeepEqual(crashMap, expected) {
				t.Errorf("incorrect processCrashLs result: expected %v, got %v\n", expected, crashMap)
			}

		},
	)
}
