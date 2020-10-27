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
	"github.com/sirupsen/logrus"
)

// Conn interface implements only necessary methods that are used in this
// repository on top of *rados.Conn. This keeps rest of the implementation
// clean and *rados.Conn doesn't need to show up everywhere (it being more of
// an implementation detail in reality). Also it makes mocking easier for
// unit-testing the collectors.
type Conn interface {
	MonCommand([]byte) ([]byte, string, error)
	GetPoolStats(string) (*rados.PoolStat, error)
}

// RadosConn implements the Conn interface with the underlying *rados.Conn
// that talks to a real Ceph cluster.
type RadosConn struct {
	user       string
	configFile string
	timeout    time.Duration
	logger     *logrus.Logger
}

// *RadosConn must implement the Conn.
var _ Conn = &RadosConn{}

// NewRadosConn returns a new RadosConn. Unlike the native rados.Conn, there
// is no need to manage the connection before/after talking to the rados; it
// is the responsibility of this *RadosConn to manage the connection.
func NewRadosConn(user, configFile string, timeout time.Duration, logger *logrus.Logger) *RadosConn {
	return &RadosConn{
		user:       user,
		configFile: configFile,
		timeout:    timeout,
		logger:     logger,
	}
}

// newRadosConn creates an established rados connection to the Ceph cluster
// using the provided Ceph user and configFile. Ceph parameters
// rados_osd_op_timeout and rados_mon_op_timeout are specified by the timeout
// value, where 0 means no limit.
func (c *RadosConn) newRadosConn() (*rados.Conn, error) {
	conn, err := rados.NewConnWithUser(c.user)
	if err != nil {
		return nil, fmt.Errorf("error creating rados connection: %s", err)
	}

	err = conn.ReadConfigFile(c.configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %s", err)
	}

	tv := strconv.FormatFloat(c.timeout.Seconds(), 'f', -1, 64)
	// Set rados_osd_op_timeout and rados_mon_op_timeout to avoid Mon
	// and PG command hang.
	// See
	// https://github.com/ceph/ceph/blob/d4872ce97a2825afcb58876559cc73aaa1862c0f/src/common/legacy_config_opts.h#L1258-L1259
	err = conn.SetConfigOption("rados_osd_op_timeout", tv)
	if err != nil {
		return nil, fmt.Errorf("error setting rados_osd_op_timeout: %s", err)
	}

	err = conn.SetConfigOption("rados_mon_op_timeout", tv)
	if err != nil {
		return nil, fmt.Errorf("error setting rados_mon_op_timeout: %s", err)
	}

	err = conn.Connect()
	if err != nil {
		return nil, fmt.Errorf("error connecting to rados: %s", err)
	}

	return conn, nil
}

// MonCommand executes a monitor command to rados.
func (c *RadosConn) MonCommand(args []byte) (buffer []byte, info string, err error) {
	ll := c.logger.WithField("args", string(args))

	ll.Trace("creating rados connection to execute mon command")

	conn, err := c.newRadosConn()
	if err != nil {
		return nil, "", err
	}
	defer conn.Shutdown()

	ll = ll.WithField("conn", conn.GetInstanceID())

	ll.Trace("start executing mon command")

	buffer, info, err = conn.MonCommand(args)

	ll.WithError(err).Trace("complete executing mon command")

	return
}

// GetPoolStats returns a *rados.PoolStat for the given rados pool.
func (c *RadosConn) GetPoolStats(pool string) (stat *rados.PoolStat, err error) {
	ll := c.logger.WithField("pool", pool)

	ll.Trace("creating rados connection to get pool stats")

	conn, err := c.newRadosConn()
	if err != nil {
		return nil, err
	}
	defer conn.Shutdown()

	ll = ll.WithField("conn", conn.GetInstanceID())

	ll.Trace("opening IOContext for pool")

	ioCtx, err := conn.OpenIOContext(pool)
	if err != nil {
		return nil, err
	}
	defer ioCtx.Destroy()

	ll.Trace("start getting pool stats")

	st, err := ioCtx.GetPoolStats()
	if err != nil {
		stat = nil
	} else {
		stat = &st
	}

	ll.WithError(err).Trace("complete getting pool stats")

	return
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

// MonCommand returns the provided output string to NoopConn as is, making it
// seem like it actually ran something and produced that string as a result.
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

// GetPoolStats always returns a nil and "not implmemented" error.
func (n *NoopConn) GetPoolStats(pool string) (*rados.PoolStat, error) {
	return nil, fmt.Errorf("not implemented")
}
