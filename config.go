package main

import (
	"io/ioutil"
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
	cfgData, err := ioutil.ReadFile(p)
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
