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

package ceph

import (
	"bytes"
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
	MgrCommand([][]byte) ([]byte, string, error)
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

// MgrCommand executes a manager command to rados.
func (c *RadosConn) MgrCommand(args [][]byte) (buffer []byte, info string, err error) {
	ll := c.logger.WithField("args", string(bytes.Join(args, []byte(","))))

	ll.Trace("creating rados connection to execute mgr command")

	conn, err := c.newRadosConn()
	if err != nil {
		return nil, "", err
	}
	defer conn.Shutdown()

	ll = ll.WithField("conn", conn.GetInstanceID())

	ll.Trace("start executing mgr command")

	buffer, info, err = conn.MgrCommand(args)

	ll.WithError(err).Trace("complete executing mgr command")

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
