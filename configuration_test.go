package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfiguration(t *testing.T) {

	t.Run("nominal", func(t *testing.T) {
		cfg, err := LoadConfiguration("./testdata/" + t.Name() + ".json")

		assert.NoError(t, err)
		assert.Equal(t, cfg, Configuration{
			Environments: map[string]Environment{
				"default": {
					"token": "default-token",
				},
				"test": {
					"token": "test-token",
				},
			},
		})
	})

	t.Run("no config file", func(t *testing.T) {
		cfg, err := LoadConfiguration(DEFAULT_CONFIG_FILE_PATH)

		assert.NoError(t, err)
		assert.Equal(t, cfg, defaultConfig)
	})
}

func TestConfiguration_Env(t *testing.T) {
	cfg := Configuration{
		Environments: map[string]Environment{
			"default": {
				"foo": "bar",
			},
			"custom": {
				"foo": "baz",
			},
		},
	}

	env, err := cfg.Env("custom")

	assert.NoError(t, err)
	assert.Equal(t, cfg.Environments["custom"], env)
}
