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
			// Example with the full output, further examples will be simpler
			name: "single new crash",
			input: `
[
	{
		"os_version_id": "7",
		"assert_condition": "p != obs_call_gate.end()",
		"utsname_release": "5.10.53-138-generic",
		"os_name": "CentOS Linux",
		"entity_name": "client.admin",
		"assert_file": "/ceph/src/common/config_proxy.h",
		"timestamp": "2022-01-25 21:03:38.371403Z",
		"process_name": "rbd-nbd",
		"utsname_machine": "x86_64",
		"utsname_sysname": "Linux",
		"os_version": "7 (Core)",
		"os_id": "centos",
		"assert_thread_name": "rbd-nbd",
		"utsname_version": "#4745ab954 SMP Fri Oct 22 23:05:54 UTC 2021",
		"backtrace": [
		  "(()+0xe54d4) [0x5561b4a744d4]",
		  "(()+0xf630) [0x7f18aac9f630]",
		  "(gsignal()+0x37) [0x7f18a9256387]",
		  "(abort()+0x148) [0x7f18a9257a78]",
		  "(ceph::__ceph_assert_fail(char const*, char const*, int, char const*)+0x199) [0x7f18ac7dce46]",
		  "(()+0x25cfbf) [0x7f18ac7dcfbf]",
		  "(ConfigProxy::call_gate_enter(ceph::md_config_obs_impl<ConfigProxy>*)+0x79) [0x5561b4a6cc67]",
		  "(ConfigProxy::map_observer_changes(ceph::md_config_obs_impl<ConfigProxy>*, std::string const&, std::map<ceph::md_config_obs_impl<ConfigProxy>*, std::set<std::string, std::less<std::string>, std::allocator<std::string> >, std::less<ceph::md_config_obs_impl<ConfigProxy>*>, std::allocator<std::pair<ceph::md_config_obs_impl<ConfigProxy>* const, std::set<std::string, std::less<std::string>, std::allocator<std::string> > > > >*)+0x120) [0x5561b4a6d0a2]",
		  "(ConfigProxy::_gather_changes(std::set<std::string, std::less<std::string>, std::allocator<std::string> >&, std::map<ceph::md_config_obs_impl<ConfigProxy>*, std::set<std::string, std::less<std::string>, std::allocator<std::string> >, std::less<ceph::md_config_obs_impl<ConfigProxy>*>, std::allocator<std::pair<ceph::md_config_obs_impl<ConfigProxy>* const, std::set<std::string, std::less<std::string>, std::allocator<std::string> > > > >*, std::ostream*)::{lambda(ceph::md_config_obs_impl<ConfigProxy>*, std::string const&)#1}::operator()(ceph::md_config_obs_impl<ConfigProxy>*, std::string const&) const+0x33) [0x5561b4a6d651]",
		  "(std::_Function_handler<void (ceph::md_config_obs_impl<ConfigProxy>*, std::string const&), ConfigProxy::_gather_changes(std::set<std::string, std::less<std::string>, std::allocator<std::string> >&, std::map<ceph::md_config_obs_impl<ConfigProxy>*, std::set<std::string, std::less<std::string>, std::allocator<std::string> >, std::less<ceph::md_config_obs_impl<ConfigProxy>*>, std::allocator<std::pair<ceph::md_config_obs_impl<ConfigProxy>* const, std::set<std::string, std::less<std::string>, std::allocator<std::string> > > > >*, std::ostream*)::{lambda(ceph::md_config_obs_impl<ConfigProxy>*, std::string const&)#1}>::_M_invoke(std::_Any_data const&, ceph::md_config_obs_impl<ConfigProxy>*&&, std::string const&)+0x52) [0x5561b4a6f11e]",
		  "(std::function<void (ceph::md_config_obs_impl<ConfigProxy>*, std::string const&)>::operator()(ceph::md_config_obs_impl<ConfigProxy>*, std::string const&) const+0x61) [0x5561b4a6f05f]",
		  "(void ObserverMgr<ceph::md_config_obs_impl<ConfigProxy> >::for_each_change<ConfigProxy>(std::set<std::string, std::less<std::string>, std::allocator<std::string> > const&, ConfigProxy&, std::function<void (ceph::md_config_obs_impl<ConfigProxy>*, std::string const&)>, std::ostream*)+0x1cb) [0x5561b4a6e343]",
		  "(ConfigProxy::_gather_changes(std::set<std::string, std::less<std::string>, std::allocator<std::string> >&, std::map<ceph::md_config_obs_impl<ConfigProxy>*, std::set<std::string, std::less<std::string>, std::allocator<std::string> >, std::less<ceph::md_config_obs_impl<ConfigProxy>*>, std::allocator<std::pair<ceph::md_config_obs_impl<ConfigProxy>* const, std::set<std::string, std::less<std::string>, std::allocator<std::string> > > > >*, std::ostream*)+0x76) [0x5561b4a6d6ca]",
		  "(ConfigProxy::apply_changes(std::ostream*)+0x7c) [0x5561b4a6d5aa]",
		  "(global_init(std::map<std::string, std::string, std::less<std::string>, std::allocator<std::pair<std::string const, std::string> > > const*, std::vector<char const*, std::allocator<char const*> >&, unsigned int, code_environment_t, int, char const*, bool)+0x1022) [0x5561b4a6a806]",
		  "(()+0x9380c) [0x5561b4a2280c]",
		  "(()+0x9618a) [0x5561b4a2518a]",
		  "(main()+0x20) [0x5561b4a252e4]",
		  "(__libc_start_main()+0xf5) [0x7f18a9242555]",
		  "(()+0x907f9) [0x5561b4a1f7f9]"
		],
		"utsname_hostname": "test-ceph-server.company.example",
		"assert_msg": "/ceph/src/common/config_proxy.h: In function 'void ConfigProxy::call_gate_enter(ConfigProxy::md_config_obs_t*)' thread 7f18b63dfa00 time 2022-01-25 21:03:38.368357\n/ceph/src/common/config_proxy.h: 65: FAILED ceph_assert(p != obs_call_gate.end())\n",
		"crash_id": "2022-01-25_21:03:38.371403Z_f9df5b64-32ef-4073-8b37-d1c5a1b3dcb8",
		"assert_line": 65,
		"ceph_version": "14.2.18",
		"assert_func": "void ConfigProxy::call_gate_enter(ConfigProxy::md_config_obs_t*)"
	}
]`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="client.admin",hostname="test-ceph-server.company.example",status="new"} 1`),
			},
		},
		{
			name: "single archived crash",
			input: `
[
	{
		"entity_name": "client.admin",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-01-25 21:02:46.687015Z",
		"archived": "2022-06-14 19:44:40.356826",
		"crash_id": "2022-01-25_21:02:46.687015Z_d6513591-c16b-472f-8d40-5a143b28837d"
	}
]
			`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="client.admin",hostname="test-ceph-server.company.example",status="archived"} 1`),
			},
		},
		{
			name: "two new crashes same entity",
			input: `
[
	{
		"entity_name": "osd.0",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-02-01 21:02:46.687015Z",
		"crash_id": "2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9"
	},
	{
		"entity_name": "osd.0",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-02-03 04:05:45.419226Z",
		"crash_id": "2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a"
	}
]`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="osd.0",hostname="test-ceph-server.company.example",status="new"} 2`),
			},
		},
		{
			name: "mix of crashes same entity",
			input: `
[
	{
		"entity_name": "osd.0",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-02-01 21:02:46.687015Z",
		"crash_id": "2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9"
	},
	{
		"entity_name": "osd.0",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-02-03 04:05:45.419226Z",
		"archived": "2022-06-14 19:44:40.356826",
		"crash_id": "2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a"
	}
]`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="osd.0",hostname="test-ceph-server.company.example",status="new"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="osd.0",hostname="test-ceph-server.company.example",status="archived"} 1`),
			},
		},
		{
			name: "mix of crashes different entities",
			input: `
[
	{
		"entity_name": "mgr.mgr-node-01",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-02-01 21:02:46.687015Z",
		"crash_id": "2022-02-01_21:02:46.687015Z_0de8b741-b323-4f63-828a-e460294e28b9"
	},
	{
		"entity_name": "client.admin",
		"utsname_hostname": "test-ceph-server.company.example",
		"timestamp": "2022-02-03 04:05:45.419226Z",
		"crash_id": "2022-02-03_04:05:45.419226Z_11c639af-5eb2-4a29-91aa-20120218891a"
	}
]`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="mgr.mgr-node-01",hostname="test-ceph-server.company.example",status="new"} 1`),
				regexp.MustCompile(`crash_reports{cluster="ceph",entity="client.admin",hostname="test-ceph-server.company.example",status="new"} 1`),
			},
		},
		{
			// At least code shouldn't panic
			name:    "no crashes",
			input:   `[]`,
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
