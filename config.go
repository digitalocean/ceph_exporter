//   Copyright 2024 DigitalOcean
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

package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

type ClusterConfig struct {
	ClusterLabel string `yaml:"cluster_label"`
	User         string `yaml:"user"`
	ConfigFile   string `yaml:"config_file"`
}

// Config is the top-level configuration for Metastord.
type Config struct {
	Cluster []*ClusterConfig
}

// fileExists returns true if the path exists and is a file.
func fileExists(path string) bool {
	stat, err := os.Stat(path)
	return !os.IsNotExist(err) && !stat.IsDir()
}

func ParseConfig(p string) (*Config, error) {
	cfgData, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(cfgData, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
