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

func TestPoolUsageCollector(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	for _, tt := range []struct {
		input              string
		reMatch, reUnmatch []*regexp.Regexp
	}{
		{
			input: `
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "objects": 5, "rd": 4, "wr": 6}}
]}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 20`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 5`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 4`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 6`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"objects": 5, "rd": 4, "wr": 6}}
]}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 0`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 5`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 4`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 6`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "rd": 4, "wr": 6}}
]}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 20`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 0`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 4`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 6`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "objects": 5, "wr": 6}}
]}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 20`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 5`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 0`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 6`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "objects": 5, "rd": 4}}
]}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 20`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 5`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 4`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 0`),
			},
			reUnmatch: []*regexp.Regexp{},
		},
		{
			input: `
{"pools": [
    {{{{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "objects": 5, "rd": 4, "wr": 6}}
]}`,
			reMatch: []*regexp.Regexp{},
			reUnmatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes`),
				regexp.MustCompile(`pool_objects_total`),
				regexp.MustCompile(`pool_read_total`),
				regexp.MustCompile(`pool_write_total`),
			},
		},
		{
			input: `
{"pools": [
	{"name": "rbd", "id": 11, "stats": {"bytes_used": 20, "objects": 5, "rd": 4, "wr": 6}},
	{"name": "rbd-new", "id": 12, "stats": {"bytes_used": 50, "objects": 20, "rd": 10, "wr": 30}}
]}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`pool_used_bytes{pool="rbd"} 20`),
				regexp.MustCompile(`pool_objects_total{pool="rbd"} 5`),
				regexp.MustCompile(`pool_read_total{pool="rbd"} 4`),
				regexp.MustCompile(`pool_write_total{pool="rbd"} 6`),
				regexp.MustCompile(`pool_used_bytes{pool="rbd-new"} 50`),
				regexp.MustCompile(`pool_objects_total{pool="rbd-new"} 20`),
				regexp.MustCompile(`pool_read_total{pool="rbd-new"} 10`),
				regexp.MustCompile(`pool_write_total{pool="rbd-new"} 30`),
			},
			reUnmatch: []*regexp.Regexp{},
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
