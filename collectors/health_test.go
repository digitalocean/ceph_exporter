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
			input: `
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
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`degraded_pgs{cluster="ceph"} 5`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "15 pgs stuck degraded"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`stuck_degraded_pgs{cluster="ceph"} 15`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "6 pgs unclean"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`unclean_pgs{cluster="ceph"} 6`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "16 pgs stuck unclean"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`stuck_unclean_pgs{cluster="ceph"} 16`),
			},
		},
		{
			input: `
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
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`undersized_pgs{cluster="ceph"} 7`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "17 pgs stuck undersized"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`stuck_undersized_pgs{cluster="ceph"} 17`),
			},
		},
		{
			input: `
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
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`stale_pgs{cluster="ceph"} 8`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "18 pgs stuck stale"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`stuck_stale_pgs{cluster="ceph"} 18`),
			},
		},
		{
			input: `
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
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`degraded_objects{cluster="ceph"} 10`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "recovery 20/40 objects misplaced"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`misplaced_objects{cluster="ceph"} 20`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 20,
			"num_up_osds": 10,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`osds_down{cluster="ceph"} 10`),
			},
		},
		{
			input: `
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
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`osds{cluster="ceph"} 1200`),
				regexp.MustCompile(`osds_up{cluster="ceph"} 1200`),
				regexp.MustCompile(`osds_in{cluster="ceph"} 1190`),
				regexp.MustCompile(`pgs_remapped{cluster="ceph"} 10`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 1200,
			"num_up_osds": 1200,
			"num_in_osds": 1190,
			"num_remapped_pgs": 10
		}
	},
	"health": { "overall_status": "HEALTH_OK" } }`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 0`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 1200,
			"num_up_osds": 1200,
			"num_in_osds": 1190,
			"num_remapped_pgs": 10
		}
	},
	"health": { "overall_status": "HEALTH_WARN", "status": "HEALTH_OK } }`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 0`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 1200,
			"num_up_osds": 1200,
			"num_in_osds": 1190,
			"num_remapped_pgs": 10
		}
	},
	"health": { "status": "HEALTH_OK } }`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 0`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 1200,
			"num_up_osds": 1200,
			"num_in_osds": 1190,
			"num_remapped_pgs": 10
		}
	},
	"health": { "overall_status": "HEALTH_WARN" } }`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 1`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 1200,
			"num_up_osds": 1200,
			"num_in_osds": 1190,
			"num_remapped_pgs": 10
		}
	},
	"health": { "overall_status": "HEALTH_ERR" } }`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 2`),
			},
		},
		{
			input: `
$ sudo ceph -s
    cluster eff51be8-938a-4afa-b0d1-7a580b4ceb37
     health HEALTH_OK
     monmap e3: 3 mons at {mon01,mon02,mon03}
  recovery io 5779 MB/s, 4 keys/s, 1522 objects/s
  client io 4273 kB/s rd, 2740 MB/s wr, 2863 op/s
`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`recovery_io_bytes{cluster="ceph"} 5.779e`),
				regexp.MustCompile(`recovery_io_keys{cluster="ceph"} 4`),
				regexp.MustCompile(`recovery_io_objects{cluster="ceph"} 1522`),
				regexp.MustCompile(`client_io_ops{cluster="ceph"} 2863`),
				regexp.MustCompile(`client_io_read_bytes{cluster="ceph"} 4.273e`),
				regexp.MustCompile(`client_io_write_bytes{cluster="ceph"} 2.74e`),
			},
		},
		{
			input: `
$ sudo ceph -s
    cluster eff51be8-938a-4afa-b0d1-7a580b4ceb37
     health HEALTH_OK
     monmap e3: 3 mons at {mon01,mon02,mon03}
  recovery io 5779 MB/s, 4 keys/s, 1522 objects/s
  client io 2863 op/s rd, 5847 op/s wr
  cache io 251 MB/s flush, 6646 kB/s evict, 55 op/s promote
`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`recovery_io_bytes{cluster="ceph"} 5.779e`),
				regexp.MustCompile(`recovery_io_keys{cluster="ceph"} 4`),
				regexp.MustCompile(`recovery_io_objects{cluster="ceph"} 1522`),
				regexp.MustCompile(`client_io_ops{cluster="ceph"} 8710`),
				regexp.MustCompile(`client_io_read_ops{cluster="ceph"} 2863`),
				regexp.MustCompile(`client_io_write_ops{cluster="ceph"} 5847`),
				regexp.MustCompile(`cache_flush_io_bytes{cluster="ceph"} 2.51e`),
				regexp.MustCompile(`cache_evict_io_bytes{cluster="ceph"} 6.646e`),
				regexp.MustCompile(`cache_promote_io_ops{cluster="ceph"} 55`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"pgmap": { "num_pgs": 52000 },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "7 pgs undersized"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`total_pgs{cluster="ceph"} 52000`),
			},
		},
		{
			input: `
{
	"osdmap": {
		"osdmap": {
			"num_osds": 0,
			"num_up_osds": 0,
			"num_in_osds": 0,
			"num_remapped_pgs": 0
		}
	},
	"pgmap": {
		"pgs_by_state": [
			{
				"state_name": "active+clean+scrubbing",
				"count": 2
			},
			{
				"state_name": "active+clean+scrubbing+deep",
				"count": 5
			}
		],
		"num_pgs": 52000
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "7 pgs undersized"}]}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`active_pgs{cluster="ceph"} 7`),
				regexp.MustCompile(`scrubbing_pgs{cluster="ceph"} 2`),
				regexp.MustCompile(`deep_scrubbing_pgs{cluster="ceph"} 5`),
			},
		},
		{
			input: `
{
	"health": {
		"summary": [
			{
				"severity": "HEALTH_WARN",
				"summary": "6 requests are blocked > 32 sec"
			}
		]
	}
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests{cluster="ceph"} 6`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "REQUEST_SLOW": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "6 slow requests are blocked > 32 sec"
        }
      }
    }
  }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests{cluster="ceph"} 6`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "REQUEST_STUCK": {
        "severity": "HEALTH_ERR",
        "summary": {
          "message": "125 stuck requests are blocked > 4194.3 sec"
        }
      }
    }
  }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`stuck_requests{cluster="ceph"} 125`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "PG_DEGRADED": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "Degraded data redundancy: 154443937/17497658377 objects degraded (0.883%), 4886 pgs unclean, 4317 pgs degraded, 516 pgs undersized"
        }
      }
    }
  }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`degraded_objects{cluster="ceph"} 1.54443937e\+08`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "OBJECT_MISPLACED": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "431295341/17497658377 objects misplaced (2.465%)"
        }
      }
    }
  }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`misplaced_objects{cluster="ceph"} 4.31295341e\+08`),
			},
		},
		{
			input: `
{
  	"checks": {
		"REQUEST_STUCK": {
			"detail": [
				{
					"message": "413 ops are blocked > 4194.3 sec"
				},
				{
					"message": "osd.131 has stuck requests > 4194.3 sec"
				}
			],
			"summary": {
				"message": "413 stuck requests are blocked > 4096 sec"
			},
			"severity": "HEALTH_ERR"
		},
    	"REQUEST_SLOW": {
			"severity": "HEALTH_WARN",
			"summary": {
				"message": "286 slow requests are blocked > 32 sec"
			},
			"detail": [
				{
					"message": "102 ops are blocked > 524.288 sec"
				},
				{
					"message": "84 ops are blocked > 262.144 sec"
				},
				{
					"message": "53 ops are blocked > 131.072 sec"
				},
				{
					"message": "33 ops are blocked > 65.536 sec"
				},
				{
					"message": "14 ops are blocked > 32.768 sec"
				},
				{
					"message": "osds 363,463 have blocked requests > 32.768 sec"
				},
				{
					"message": "osd.349 has blocked requests > 524.288 sec"
				}
			]
      	}
    }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="363"} 14`),
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="463"} 14`),
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="349"} 272`),
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="131"} 413`),
			},
		},
		{
			input: `
{
  	"checks": {
    	"REQUEST_SLOW": {
			"severity": "HEALTH_WARN",
			"summary": {
				"message": "286 slow requests are blocked > 32 sec"
			},
			"detail": [
				{
					"message": "102 ops are blocked > 524.288 sec"
				},
				{
					"message": "84 ops are blocked > 262.144 sec"
				},
				{
					"message": "53 ops are blocked > 131.072 sec"
				},
				{
					"message": "33 ops are blocked > 65.536 sec"
				},
				{
					"message": "14 ops are blocked > 32.768 sec"
				},
				{
					"message": "osds 363,463 have blocked requests > 131.072 sec"
				},
				{
					"message": "osd.349 has blocked requests > 524.288 sec"
				}
			]
      	}
    }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="363"} 100`),
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="463"} 100`),
				regexp.MustCompile(`slow_requests_osd{cluster="ceph",osd="349"} 186`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "OSDMAP_FLAGS": {
        "severity": "HEALTH_WARN",
        "summary": {
            "message": "pauserd,pausewr,noout,noin,norecover,noscrub,notieragent flag(s) set; mon 482f68d873d2 is low on available space"
	}
      }
    }
  }
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`osdmap_flag_full{cluster="ceph"} 0`),
				regexp.MustCompile(`osdmap_flag_pauserd{cluster="ceph"} 1`),
				regexp.MustCompile(`osdmap_flag_pausewr{cluster="ceph"} 1`),
				regexp.MustCompile(`osdmap_flag_noup{cluster="ceph"} 0`),
				regexp.MustCompile(`osdmap_flag_nodown{cluster="ceph"} 0`),
				regexp.MustCompile(`osdmap_flag_noin{cluster="ceph"} 1`),
				regexp.MustCompile(`osdmap_flag_noout{cluster="ceph"} 1`),
				regexp.MustCompile(`osdmap_flag_nobackfill{cluster="ceph"} 0`),
				regexp.MustCompile(`osdmap_flag_norecover{cluster="ceph"} 1`),
				regexp.MustCompile(`osdmap_flag_norebalance{cluster="ceph"} 0`),
				regexp.MustCompile(`osdmap_flag_noscrub{cluster="ceph"} 1`),
				regexp.MustCompile(`osdmap_flag_nodeep_scrub{cluster="ceph"} 0`),
				regexp.MustCompile(`osdmap_flag_notieragent{cluster="ceph"} 1`),
			},
		},
		{
			input: `
{
	"pgmap": {
		"write_op_per_sec": 500,
		"read_op_per_sec": 1000,
		"write_bytes_sec": 829694017,
		"read_bytes_sec": 941980516,
		"degraded_ratio": 0.213363,
		"degraded_total": 7488077,
		"degraded_objects": 1597678,
		"pgs_by_state": [
			{
				"count": 10,
				"state_name": "active+undersized+degraded"
			},
			{
				"count": 20,
				"state_name": "active+clean"
			},
			{
				"count": 10,
				"state_name": "undersized+degraded+peered"
			},
			{
				"count": 20,
				"state_name": "activating+undersized+degraded"
			},
			{
				"count": 30,
				"state_name": "activating+stale+unclean"
			},
			{
				"count": 10,
				"state_name": "peering"
			},
			{
				"count": 20,
				"state_name": "scrubbing"
			},
			{
				"count": 10,
				"state_name": "scrubbing+deep"
			},
            {
                "state_name": "remapped+recovering",
                "count": 5
            },
            {
                "state_name": "active+remapped+backfilling",
                "count": 2
            },
            {
                "state_name": "recovery_wait+inconsistent",
                "count": 2
            },
            {
                "state_name": "recovery_wait+remapped",
                "count": 1
            },
            {
                "state_name": "active+undersized+remapped+backfill_wait",
                "count": 1
            },
            {
                "state_name": "active+undersized+remapped+backfill_wait+forced_backfill",
                "count": 10
            },
            {
                "state_name": "down",
                "count": 6
            },
            {
                "state_name": "down+remapped",
                "count": 31
            },
            {
                "state_name": "active+forced_recovery+undersized",
                "count": 1
            }
		],
		"num_pgs": 9208,
		"num_pools": 29,
		"num_objects": 1315631,
		"data_bytes": 1230716754607,
		"recovering_objects_per_sec": 140,
		"recovering_bytes_per_sec": 65536,
		"recovering_keys_per_sec": 25,
		"flush_bytes_sec": 59300,
		"evict_bytes_sec": 3000,
		"promote_op_per_sec": 1000,
		"bytes_used": 1861238087680,
		"bytes_avail": 2535859327381504,
		"bytes_total": 2537720565469184
	}	
}`,
			regexes: []*regexp.Regexp{
				regexp.MustCompile(`active_pgs{cluster="ceph"} 44`),
				regexp.MustCompile(`degraded_pgs{cluster="ceph"} 40`),
				regexp.MustCompile(`unclean_pgs{cluster="ceph"} 30`),
				regexp.MustCompile(`undersized_pgs{cluster="ceph"} 52`),
				regexp.MustCompile(`stale_pgs{cluster="ceph"} 30`),
				regexp.MustCompile(`peering_pgs{cluster="ceph"} 10`),
				regexp.MustCompile(`scrubbing_pgs{cluster="ceph"} 20`),
				regexp.MustCompile(`deep_scrubbing_pgs{cluster="ceph"} 10`),
				regexp.MustCompile(`recovering_pgs{cluster="ceph"} 5`),
				regexp.MustCompile(`recovery_wait_pgs{cluster="ceph"} 3`),
				regexp.MustCompile(`backfilling_pgs{cluster="ceph"} 2`),
				regexp.MustCompile(`backfill_wait_pgs{cluster="ceph"} 11`),
				regexp.MustCompile(`forced_recovery_pgs{cluster="ceph"} 1`),
				regexp.MustCompile(`forced_backfill_pgs{cluster="ceph"} 10`),
				regexp.MustCompile(`down_pgs{cluster="ceph"} 37`),
				regexp.MustCompile(`recovery_io_bytes{cluster="ceph"} 65536`),
				regexp.MustCompile(`recovery_io_keys{cluster="ceph"} 25`),
				regexp.MustCompile(`recovery_io_objects{cluster="ceph"} 140`),
				regexp.MustCompile(`client_io_ops{cluster="ceph"} 1500`),
				regexp.MustCompile(`client_io_read_ops{cluster="ceph"} 1000`),
				regexp.MustCompile(`client_io_write_ops{cluster="ceph"} 500`),
				regexp.MustCompile(`cache_flush_io_bytes{cluster="ceph"} 59300`),
				regexp.MustCompile(`cache_evict_io_bytes{cluster="ceph"} 3000`),
				regexp.MustCompile(`cache_promote_io_ops{cluster="ceph"} 1000`),
			},
		},
	} {
		func() {
			collector := NewClusterHealthCollector(NewNoopConn(tt.input), "ceph")
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
