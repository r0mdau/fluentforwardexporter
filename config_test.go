// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/r0mdau/fluentforwardexporter/internal/metadata"
)

func TestLoadConfigNewExporter(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint: Endpoint{
						TCPAddr:               validEndpoint,
						ValidateTCPResolution: false,
					},
					ConnectionTimeout: time.Second * 30,
					ClientConfig: configtls.ClientConfig{
						Insecure:           false,
						InsecureSkipVerify: true,
						Config: configtls.Config{
							CAFile:   "ca.crt",
							CertFile: "client.crt",
							KeyFile:  "client.key",
						},
					},
					SharedKey: "otelcol-dev",
				},
				RequireAck:   true,
				Tag:          "nginx",
				CompressGzip: true,
				DefaultLabelsEnabled: map[string]bool{
					"timestamp": true,
					"level":     true,
					"message":   true,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueBatchConfig: configoptional.Some(exporterhelper.QueueBatchConfig{
					NumConsumers: 2,
					QueueSize:    10,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
					Batch:        exporterhelper.NewDefaultQueueConfig().Batch,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc   string
		modify func(cfg *Config)
		err    error
	}{
		{
			desc: "QueueSettings are invalid",
			modify: func(cfg *Config) {
				cfg.QueueBatchConfig = configoptional.Some(exporterhelper.QueueBatchConfig{
					QueueSize: -1,
				})
			},
			err: fmt.Errorf("queue settings has invalid configuration"),
		},
		{
			desc: "Endpoint is empty",
			modify: func(cfg *Config) {
				cfg.Endpoint.TCPAddr = ""
			},
			err: fmt.Errorf("endpoint.tcp_addr must not be empty"),
		},
		{
			desc: "Endpoint is invalid",
			modify: func(cfg *Config) {
				cfg.Endpoint.TCPAddr = "http://localhost:24224"
				cfg.Endpoint.ValidateTCPResolution = true
			},
			err: fmt.Errorf("exporter has an invalid TCP endpoint: address http://localhost:24224: too many colons in address"),
		},
		{
			desc: "Endpoint is invalid with ValidateTCPResolution false throw no error",
			modify: func(cfg *Config) {
				cfg.Endpoint.TCPAddr = "http://localhost:24224"
				cfg.Endpoint.ValidateTCPResolution = false
			},
			err: nil,
		},
		{
			desc: "Tag is empty",
			modify: func(cfg *Config) {
				cfg.Tag = ""
			},
			err: fmt.Errorf("tag must not be empty"),
		},
		{
			desc: "Default labels contain unknown labels",
			modify: func(cfg *Config) {
				cfg.DefaultLabelsEnabled = map[string]bool{
					"message":  true,
					"severity": true,
					"msg":      false,
				}
			},
			err: fmt.Errorf(`default_labels_enabled has unknown labels ["msg" "severity"], supported labels are ["timestamp" "level" "message"]`),
		},
		{
			desc: "Default labels can be disabled",
			modify: func(cfg *Config) {
				cfg.DefaultLabelsEnabled = map[string]bool{
					"timestamp": true,
					"level":     false,
					"message":   false,
				}
			},
			err: nil,
		},
		{
			desc: "Config is valid",
			modify: func(cfg *Config) {
				cfg.Endpoint.TCPAddr = validEndpoint
				cfg.Endpoint.ValidateTCPResolution = true
			},
			err: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tc.modify(cfg)
			err := cfg.Validate()
			if tc.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
