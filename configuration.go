package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Environment map[string]string

type Configuration struct {
	Environments map[string]Environment `json:"environments"`
}

var DEFAULT_CONFIG_FILE_PATH = "narcolepsy.json"

var defaultConfig = Configuration{
	Environments: map[string]Environment{
		"default": make(Environment),
	},
}

func LoadConfiguration(filename string) (Configuration, error) {
	var c = defaultConfig

	exists := fileExists(filename)
	isDefaultConfig := filename == DEFAULT_CONFIG_FILE_PATH

	// omitting the config file is not an error if no config path was specified
	if isDefaultConfig && !exists {
		return defaultConfig, nil
	}

	raw, err := os.ReadFile(filename)
	if err != nil {
		return Configuration{}, err
	}

	err = json.Unmarshal(raw, &c)
	if err != nil {
		return Configuration{}, err
	}

	return c, nil
}

func (c Configuration) Env(name string) (Environment, error) {
	env, ok := c.Environments[name]
	if !ok {
		return Environment{}, fmt.Errorf("unknown environment %s", name)
	}

	return env, nil
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if err != nil {
		return false
	}

	return true
}
