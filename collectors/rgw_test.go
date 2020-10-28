package collectors

import (
	"errors"
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

func TestRGWCollector(t *testing.T) {
	for _, tt := range []struct {
		input     []byte
		reMatch   []*regexp.Regexp
		reUnmatch []*regexp.Regexp
	}{
		{
			input: []byte(`
[
   {
       "tag": "00000000-0001-0000-0000-9ec86fa9a561.9695966.3129536\u0000",
       "time": "1975-01-01 16:31:09.0.564455s",
       "objs": [
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0001-0000-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           },
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0002-0000-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           }
       ]
	},
   	{
       "tag": "00000000-0002-0000-0000-9ec86fa9a561.9695966.3129536\u0000",
       "time": "1975-01-01 17:31:09.0.564455s",
       "objs": [
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0004-0000-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           },
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0005-0000-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           }
       ]
	},
   	{
       "tag": "00000000-0002-0000-0000-9ec86fa9a561.9695966.3129536\u0000",
       "time": "3075-01-01 11:30:09.0.123456s",
       "objs": [
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0001-5555-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           },
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0002-5555-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           },
           {
               "pool": "pool.rgw.buckets.data",
               "oid": "12345678-0003-5555-0000-000000000000.123456.1100__shadow_.tNcmQWnIAlJMd33ZIdhnLF9HoaY9TOv_1",
               "key": "",
               "instance": ""
           }
       ]
	}
]
`),
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rgw_gc_active_tasks{cluster="ceph"} 2`),
				regexp.MustCompile(`ceph_rgw_gc_active_objects{cluster="ceph"} 4`),
				regexp.MustCompile(`ceph_rgw_gc_pending_tasks{cluster="ceph"} 1`),
				regexp.MustCompile(`ceph_rgw_gc_pending_objects{cluster="ceph"} 3`),
			},
		},
		{
			input: []byte(`[]`),
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rgw_gc_active_tasks{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rgw_gc_active_objects{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rgw_gc_pending_tasks{cluster="ceph"} 0`),
				regexp.MustCompile(`ceph_rgw_gc_pending_objects{cluster="ceph"} 0`),
			},
		},
		{
			// force an error return json deserialization
			input: []byte(`[ { "bad-object": 17,,, ]`),
			reUnmatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rgw_gc`),
			},
		},
		{
			// force an error return from getRGWGCTaskList
			input: nil,
			reUnmatch: []*regexp.Regexp{
				regexp.MustCompile(`ceph_rgw_gc`),
			},
		},
	} {
		func() {
			collector := NewRGWCollector("ceph", "", false, logrus.New()) // run in foreground for testing
			collector.getRGWGCTaskList = func(cluster string) ([]byte, error) {
				if tt.input != nil {
					return tt.input, nil
				}
				return nil, errors.New("fake error")
			}

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

			for _, re := range tt.reUnmatch {
				require.False(t, re.Match(buf))
			}
		}()
	}
}
