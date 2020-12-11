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

	"github.com/digitalocean/ceph_exporter/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestClusterHealthCollector(t *testing.T) {
	for _, tt := range []struct {
		input   string
		reMatch []*regexp.Regexp
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
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "15 pgs stuck degraded"}]}
}`,
			reMatch: []*regexp.Regexp{
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
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "16 pgs stuck unclean"}]}
}`,
			reMatch: []*regexp.Regexp{
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
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "17 pgs stuck undersized"}]}
}`,
			reMatch: []*regexp.Regexp{
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
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "18 pgs stuck stale"}]}
}`,
			reMatch: []*regexp.Regexp{
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
	"pgmap": { "degraded_objects": 10 }
}`,
			reMatch: []*regexp.Regexp{
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
	"pgmap": { "misplaced_objects": 20 }
}`,
			reMatch: []*regexp.Regexp{
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
			reMatch: []*regexp.Regexp{
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
			reMatch: []*regexp.Regexp{
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
			reMatch: []*regexp.Regexp{
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
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 0`),
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 0`),
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
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 0`),
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 0`),
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
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 1`),
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 2`),
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
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`health_status{cluster="ceph"} 2`),
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 3`),
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
			reMatch: []*regexp.Regexp{
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
			reMatch: []*regexp.Regexp{
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
	"pgmap": { "num_pgs": 52000, "num_objects": 13156 },
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "7 pgs undersized"}]}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`total_pgs{cluster="ceph"} 52000`),
				regexp.MustCompile(`cluster_objects{cluster="ceph"} 13156`),
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
			},
			{
				"state_name": "active+clean+inconsistent",
				"count": 1
			}
		],
		"num_pgs": 52000,
		"num_objects": 13156
	},
	"health": {"summary": [{"severity": "HEALTH_WARN", "summary": "7 pgs undersized"}]}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`active_pgs{cluster="ceph"} 8`),
				regexp.MustCompile(`scrubbing_pgs{cluster="ceph"} 2`),
				regexp.MustCompile(`deep_scrubbing_pgs{cluster="ceph"} 5`),
				regexp.MustCompile(`inconsistent_pgs{cluster="ceph"} 1`),
				regexp.MustCompile(`cluster_objects{cluster="ceph"} 13156`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "MON_DOWN": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "1/3 mons down, quorum a,b"
        }
      }
    }
  }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`mons_down{cluster="ceph"} 1`),
			},
		},
		{
			input: `
{
	"health": {
		"summary": [
			{
				"severity": "SLOW_OPS",
				"summary": "3 slow ops, oldest one blocked for 1 sec, osd.39 has slow ops"
			}
		]
	}
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests{cluster="ceph"} 3`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "SLOW_OPS": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "3 slow ops, oldest one blocked for 1 sec, osd.39 has slow ops"
        }
      }
    }
  }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests{cluster="ceph"} 3`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "SLOW_OPS": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "18 slow ops, oldest one blocked for 1 sec, daemons [osd.114,osd.116,osd.33,osd.34,osd.43,osd.49,osd.53] have slow ops."
        }
      }
    }
  }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`slow_requests{cluster="ceph"} 18`),
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
  },
  "pgmap": { "degraded_objects": 154443937 }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`degraded_objects{cluster="ceph"} 1.54443937e\+08`),
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 1`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "RECENT_CRASH": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "2 daemons have recently crashed"
        }
      }
    }
  }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`new_crash_reports{cluster="ceph"} 2`),
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 1`),
			},
		},
		{
			input: `
{
  "health": {
    "checks": {
      "POOL_APP_NOT_ENABLED": {
        "severity": "HEALTH_WARN",
        "summary": {
          "message": "application not enabled on 1 pool(s)"
        }
      }
    }
  }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 2`),
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
			reMatch: []*regexp.Regexp{
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
				regexp.MustCompile(`health_status_interp{cluster="ceph"} 1`),
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
            },
            {
                "state_name": "remapped+incomplete",
                "count": 2
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
			reMatch: []*regexp.Regexp{
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
				regexp.MustCompile(`incomplete_pgs{cluster="ceph"} 2`),
				regexp.MustCompile(`recovery_io_bytes{cluster="ceph"} 65536`),
				regexp.MustCompile(`recovery_io_keys{cluster="ceph"} 25`),
				regexp.MustCompile(`recovery_io_objects{cluster="ceph"} 140`),
				regexp.MustCompile(`client_io_ops{cluster="ceph"} 1500`),
				regexp.MustCompile(`client_io_read_ops{cluster="ceph"} 1000`),
				regexp.MustCompile(`client_io_write_ops{cluster="ceph"} 500`),
				regexp.MustCompile(`cache_flush_io_bytes{cluster="ceph"} 59300`),
				regexp.MustCompile(`cache_evict_io_bytes{cluster="ceph"} 3000`),
				regexp.MustCompile(`cache_promote_io_ops{cluster="ceph"} 1000`),

				regexp.MustCompile(`pg_state{cluster="ceph",state="active"} 44`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="degraded"} 40`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="unclean"} 30`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="undersized"} 52`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="stale"} 30`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="peering"} 10`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="scrubbing"} 20`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="deep_scrubbing"} 10`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="recovering"} 5`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="recovery_wait"} 3`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="backfilling"} 2`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="backfill_wait"} 11`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="forced_recovery"} 1`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="forced_backfill"} 10`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="down"} 37`),
				regexp.MustCompile(`pg_state{cluster="ceph",state="incomplete"} 2`),
			},
		},
		{
			input: `
{
    "mgrmap": {
        "epoch": 627,
        "active_gid": 48000003,
        "active_name": "mon03",
        "active_addr": "10.0.0.3:6800/1746",
        "available": true,
        "standbys": [
            {
                "gid": 48000001,
                "name": "mon01",
                "available_modules": [
                    "balancer",
                    "dashboard",
                    "influx"
                ]
            },
            {
                "gid": 48000002,
                "name": "mon02",
                "available_modules": [
                    "balancer",
                    "dashboard",
                    "influx"
                ]
            }
        ],
        "modules": [
            "dashboard",
            "restful",
            "status"
        ],
        "available_modules": [
            "balancer",
            "dashboard",
            "influx"
        ],
        "services": {
            "dashboard": "http://mon01:7000/"
        }
    }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`mgrs_active{cluster="ceph"} 1`),
				regexp.MustCompile(`mgrs{cluster="ceph"} 3`),
			},
		},
		{
			input: `
{
    "servicemap": {
        "epoch": 30,
        "modified": "2020-07-13 22:21:53.278589",
        "services": {
            "rbd-mirror": {
                "daemons": {
                    "summary": "",
                    "681363": {
                        "start_epoch": 4,
                        "start_stamp": "2020-09-24 04:27:12.310285",
                        "addr": "10.39.70.112:0/2856123533",
                        "metadata": {
                            "arch": "x86_64",
                            "id": "prod-mon01-block01"
                        }
                    },

                    "681474": {
                        "start_epoch": 6,
                        "start_stamp": "2020-09-24 04:28:44.500861",
                        "addr": "10.39.70.111:0/3132809711",
                        "metadata": {
                            "arch": "x86_64",
                            "id": "prod-mon02-block01"
                        }
                    }
                }
            }
        }
    }
}`,
			reMatch: []*regexp.Regexp{
				regexp.MustCompile(`rbd_mirror_up{cluster="ceph",\s*name="prod-mon01-block01"} 1`),
				regexp.MustCompile(`rbd_mirror_up{cluster="ceph",\s*name="prod-mon02-block01"} 1`),
			},
		},
	} {
		func() {
			conn := &mocks.Conn{}
			conn.On("MonCommand", mock.Anything).Return(
				[]byte(tt.input), "", nil,
			)

			collector := NewClusterHealthCollector(conn, "ceph", logrus.New())
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
		}()
	}
}
