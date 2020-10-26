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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ceph/go-ceph/rados"
)

// Conn interface implements only necessary methods that are used
// in this repository of *rados.Conn. This keeps rest of the implementation
// clean and *rados.Conn doesn't need to show up everywhere (it being
// more of an implementation detail in reality). Also it makes mocking
// easier for unit-testing the collectors.
type Conn interface {
	ReadDefaultConfigFile() error
	Connect() error
	Shutdown()
	MonCommand([]byte) ([]byte, string, error)
	OpenIOContext(string) (*rados.IOContext, error)
}

// Verify that *rados.Conn implements Conn correctly.
var _ Conn = &rados.Conn{}

// CreateRadosConn creates an established rados connection to the Ceph cluster
// using the provided Ceph user and configFile. Ceph parameters
// rados_osd_op_timeout and rados_mon_op_timeout are specified by the timeout
// value, where 0 means no limit.
func CreateRadosConn(user, configFile string, timeout time.Duration) (*rados.Conn, error) {
	conn, err := rados.NewConnWithUser(user)
	if err != nil {
		return nil, fmt.Errorf("cannot create new ceph connection: %s", err)
	}

	err = conn.ReadConfigFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read ceph config file: %s", err)
	}

	tv := strconv.FormatFloat(timeout.Seconds(), 'f', -1, 64)
	// Set rados_osd_op_timeout and rados_mon_op_timeout to avoid Mon
	// and PG command hang.
	// See
	// https://github.com/ceph/ceph/blob/d4872ce97a2825afcb58876559cc73aaa1862c0f/src/common/legacy_config_opts.h#L1258-L1259
	err = conn.SetConfigOption("rados_osd_op_timeout", tv)
	if err != nil {
		return nil, fmt.Errorf("cannot set rados_osd_op_timeout for ceph cluster: %s", err)
	}

	err = conn.SetConfigOption("rados_mon_op_timeout", tv)
	if err != nil {
		return nil, fmt.Errorf("cannot set rados_mon_op_timeout for ceph cluster: %s", err)
	}

	err = conn.Connect()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to ceph cluster: %s", err)
	}

	return conn, nil
}

// NoopConn is the stub we use for mocking rados Conn. Unit testing
// each individual collectors becomes a lot easier after that.
// TODO: both output and cmdOut provide the command outputs for "go test", but
// we can deprecate output, because cmdOut is able to hold the outputs we desire
// for multiple commands for "go test".
type NoopConn struct {
	output    string // deprecated
	cmdOut    []map[string]string
	iteration int
}

// The stub we use for testing should also satisfy the interface properties.
var _ Conn = &NoopConn{}

// NewNoopConn returns an instance of *NoopConn. The string that we want output
// at the end of the command we issue to Ceph is fixed and should be specified
// in the only input parameter.
func NewNoopConn(output string) *NoopConn {
	return &NoopConn{output: output}
}

// NewNoopConnWithCmdOut returns an instance of *NoopConn. The string that we
// want output at the end of the command we issue to Ceph can be various and
// should be specified by the map in the only input parameter.
func NewNoopConnWithCmdOut(cmdOut []map[string]string) *NoopConn {
	return &NoopConn{
		cmdOut:    cmdOut,
		iteration: 0,
	}
}

// IncIteration increments iteration by 1.
func (n *NoopConn) IncIteration() {
	n.iteration++
}

// ReadDefaultConfigFile does not need to return an error. It satisfies
// rados.Conn's function with the same prototype.
func (n *NoopConn) ReadDefaultConfigFile() error {
	return nil
}

// Connect does not need to return an error. It satisfies
// rados.Conn's function with the same prototype.
func (n *NoopConn) Connect() error {
	return nil
}

// Shutdown satisfies rados.Conn's function prototype.
func (n *NoopConn) Shutdown() {}

// MonCommand returns the provided output string to NoopConn as is, making
// it seem like it actually ran something and produced that string as a result.
func (n *NoopConn) MonCommand(args []byte) ([]byte, string, error) {
	// Unmarshal the input command and see if we need to intercept
	cmd := map[string]interface{}{}
	err := json.Unmarshal(args, &cmd)
	if err != nil {
		return []byte(n.output), "", err
	}

	// Intercept and mock the output
	switch prefix := cmd["prefix"]; prefix {
	case "pg dump":
		val, ok := cmd["dumpcontents"]
		if !ok {
			break
		}

		dc, ok := val.([]interface{})
		if !ok || len(dc) == 0 {
			break
		}

		switch dc[0] {
		case "pgs_brief":
			return []byte(n.cmdOut[n.iteration]["ceph pg dump pgs_brief"]), "", nil
		}

	case "osd tree":
		val, ok := cmd["states"]
		if !ok {
			return []byte(n.cmdOut[n.iteration]["ceph osd tree"]), "", nil
		}

		st, ok := val.([]interface{})
		if !ok || len(st) == 0 {
			break
		}

		switch st[0] {
		case "down":
			return []byte(n.cmdOut[n.iteration]["ceph osd tree down"]), "", nil
		}

	case "osd df":
		return []byte(n.cmdOut[n.iteration]["ceph osd df"]), "", nil

	case "osd perf":
		return []byte(n.cmdOut[n.iteration]["ceph osd perf"]), "", nil

	case "osd dump":
		return []byte(n.cmdOut[n.iteration]["ceph osd dump"]), "", nil

	case "osd crush rule dump":
		dumpReturn :=
			`[
                           {
                             "rule_id": 0,
                             "rule_name": "replicated_rule",
                             "ruleset": 0,
                             "type": 1,
                             "min_size": 1,
                             "max_size": 10,
                             "steps": [
                               {
                                 "num": 5,
                                 "op": "set_chooseleaf_tries"
                               },
                               {
                                 "op": "take",
                                 "item": -1,
                                 "item_name": "default"
                               },
                               {
                                 "op": "chooseleaf_firstn",
                                 "num": 0,
                                 "type": "host"
                               },
                               {
                                 "op": "emit"
                               }
                             ]
                           },
                           {
                             "rule_id": 1,
                             "rule_name": "another-rule",
                             "ruleset": 1,
                             "type": 1,
                             "min_size": 1,
                             "max_size": 10,
                             "steps": [
                               {
                                 "op": "take",
                                 "item": -53,
                                 "item_name": "non-default-root"
                               },
                               {
                                 "op": "chooseleaf_firstn",
                                 "num": 0,
                                 "type": "rack"
                               },
                               {
                                 "op": "emit"
                               }
                             ]
                           }
                         ]`
		return []byte(dumpReturn), "", nil

	case "osd erasure-code-profile get":
		switch cmd["name"] {
		case "ec-4-2":
			ec42Return :=
				`{
			"crush-device-class": "",
			"crush-failure-domain": "host",
			"crush-root": "objectdata",
			"jerasure-per-chunk-alignment": "false",
			"k": "4",
			"m": "2",
			"plugin": "jerasure",
			"technique": "reed_sol_van",
			"w": "8"
		}`
			return []byte(ec42Return), "", nil

		default:
			return []byte(""), "", errors.New("unknown erasure code profile")
		}

	case "osd erasure-code-profile get replicated-ruleset":
		return []byte("{}"), "", nil
	}
	return []byte(n.output), "", nil
}

// OpenIOContext always returns a nil rados.IOContext, and "not implemented"
// error. The OpenIOContext method in the rados package returns a pointer of
// rados.IOContext that contains an actual C.rados_ioctx_t, which is not
// available in this NoopConn.
func (n *NoopConn) OpenIOContext(pool string) (*rados.IOContext, error) {
	return nil, errors.New("not implemented")
}
