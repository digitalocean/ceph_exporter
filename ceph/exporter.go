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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Jeffail/gabs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type versionedCollector interface {
	Collect(chan<- prometheus.Metric, *Version)
	Describe(chan<- *prometheus.Desc)
}

// Exporter wraps all the ceph collectors and provides a single global
// exporter to extracts metrics out of. It also ensures that the collection
// is done in a thread-safe manner, the necessary requirement stated by
// prometheus. It also implements a prometheus.Collector interface in order
// to register it correctly.
type Exporter struct {
	mu        sync.Mutex
	Conn      Conn
	Cluster   string
	Config    string
	User      string
	RgwMode   int
	RbdMirror bool
	Logger    *logrus.Logger
	Version   *Version
	cc        map[string]versionedCollector
}

// NewExporter returns an initialized *Exporter
// We can choose to enable a collector to extract stats out of by adding it to the list of collectors.
func NewExporter(conn Conn, cluster string, config string, user string, rgwMode int, logger *logrus.Logger) *Exporter {
	e := &Exporter{
		Conn:    conn,
		Cluster: cluster,
		Config:  config,
		User:    user,
		RgwMode: rgwMode,
		Logger:  logger,
	}
	err := e.setCephVersion()
	if err != nil {
		e.Logger.WithError(err).Error("failed to set ceph version")
		return nil
	}
	e.cc = e.initCollectors()

	return e
}

func (exporter *Exporter) initCollectors() map[string]versionedCollector {
	standardCollectors := map[string]versionedCollector{
		"clusterUsage":  NewClusterUsageCollector(exporter),
		"poolUsage":     NewPoolUsageCollector(exporter),
		"poolInfo":      NewPoolInfoCollector(exporter),
		"clusterHealth": NewClusterHealthCollector(exporter),
		"mon":           NewMonitorCollector(exporter),
		"osd":           NewOSDCollector(exporter),
		"crashes":       NewCrashesCollector(exporter),
	}

	switch exporter.RgwMode {
	case RGWModeForeground:
		standardCollectors["rgw"] = NewRGWCollector(exporter, false)
	case RGWModeBackground:
		standardCollectors["rgw"] = NewRGWCollector(exporter, true)
	case RGWModeDisabled:
		// nothing to do
	default:
		exporter.Logger.WithField("RgwMode", exporter.RgwMode).Warn("RGW collector disabled due to invalid mode")
	}

	return standardCollectors
}

func (exporter *Exporter) cephVersionCmd() []byte {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "version",
		"format": "json",
	})
	if err != nil {
		exporter.Logger.WithError(err).Panic("failed to marshal ceph version command")
	}

	return cmd
}

func CephVersionsCmd() ([]byte, error) {
	// Ceph versions
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "versions",
		"format": "json",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ceph versions command: %s", err)
	}
	return cmd, nil
}

func ParseCephVersions(buf []byte) (map[string]map[string]float64, error) {
	// Rather than a dedicated type, have dynamic daemons and versions
	// {"daemon": {"version1": 123, "version2": 234}}
	parsed, err := gabs.ParseJSON(buf)
	if err != nil {
		return nil, err
	}

	parsedMap, err := parsed.ChildrenMap()
	if err != nil {
		return nil, err
	}

	versions := make(map[string]map[string]float64)
	for daemonKey, innerObj := range parsedMap {
		// Read each daemon, and overall counts
		versionMap, err := innerObj.ChildrenMap()
		if err == gabs.ErrNotObj {
			continue
		} else if err != nil {
			return nil, err
		}

		versions[daemonKey] = make(map[string]float64)
		for version, countContainer := range versionMap {
			count, ok := countContainer.Data().(float64)
			if ok {
				versions[daemonKey][version] = count
			}
		}
	}

	return versions, nil
}

func (exporter *Exporter) setRbdMirror() error {

	cmd, err := CephVersionsCmd()
	if err != nil {
		exporter.Logger.WithError(err).Panic("failed to marshal ceph versions command")
	}

	buf, _, err := exporter.Conn.MonCommand(cmd)
	if err != nil {
		exporter.Logger.WithError(err).WithField(
			"args", string(cmd),
		).Error("error executing mon command")
		return err
	}

	versions, err := ParseCephVersions(buf)
	if err != nil {
		return err
	}

	// check to see if rbd-mirror is in ceph version output and not empty
	if _, exists := versions["rbd-mirror"]; exists {
		if len(versions["rbd-mirror"]) > 0 {
			if _, ok := exporter.cc["rbdMirror"]; !ok {
				exporter.cc["rbdMirror"] = NewRbdMirrorStatusCollector(exporter)
			}
		}
	} else {
		// remove the rbdMirror collector if present
		delete(exporter.cc, "rbdMirror")
	}

	return nil
}

func (exporter *Exporter) setCephVersion() error {
	buf, _, err := exporter.Conn.MonCommand(exporter.cephVersionCmd())
	if err != nil {
		return err
	}

	cephVersion := &struct {
		Version string `json:"Version"`
	}{}

	err = json.Unmarshal(buf, cephVersion)
	if err != nil {
		return err
	}

	parsedVersion, err := ParseCephVersion(cephVersion.Version)
	if err != nil {
		exporter.Logger.Info("version " + cephVersion.Version)
		return err
	}

	exporter.Version = parsedVersion

	return nil
}

// Describe sends all the descriptors of the collectors included to
// the provided channel.
func (exporter *Exporter) Describe(ch chan<- *prometheus.Desc) {
	err := exporter.setCephVersion()
	if err != nil {
		exporter.Logger.WithError(err).Error("failed to set ceph Version")
		return
	}

	err = exporter.setRbdMirror()
	if err != nil {
		exporter.Logger.WithError(err).Error("failed to set rbd mirror")
		return
	}

	for _, cc := range exporter.cc {
		cc.Describe(ch)
	}
}

// Collect sends the collected metrics from each of the collectors to
// prometheus. Collect could be called several times concurrently
// and thus its run is protected by a single mutex.
func (exporter *Exporter) Collect(ch chan<- prometheus.Metric) {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()

	err := exporter.setCephVersion()
	if err != nil {
		exporter.Logger.WithError(err).Error("failed to set ceph Version")
		return
	}

	err = exporter.setRbdMirror()
	if err != nil {
		exporter.Logger.WithError(err).Error("failed to set rbd mirror")
		return
	}

	wg := &sync.WaitGroup{}
	for _, cc := range exporter.cc {
		wg.Add(1)
		go func(cc versionedCollector, wg *sync.WaitGroup) {
			cc.Collect(ch, exporter.Version)
			wg.Done()
		}(cc, wg)
	}
	wg.Wait()
}
