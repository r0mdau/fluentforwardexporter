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
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/axoflow/fluentforwardexporter/internal/metadata"
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
					Endpoint:          validEndpoint,
					ConnectionTimeout: time.Second * 30,
					TLSSetting: configtls.ClientConfig{
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
					"time":     true,
					"exporter": false,
					"job":      false,
					"instance": false,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
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

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc string
		cfg  *Config
		err  error
	}{
		{
			desc: "QueueSettings are invalid",
			cfg:  &Config{QueueSettings: exporterhelper.QueueSettings{QueueSize: -1, Enabled: true}},
			err:  fmt.Errorf("queue settings has invalid configuration"),
		},
		{
			desc: "Endpoint is invalid",
			cfg: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint:          "http://localhost:24224",
					ConnectionTimeout: time.Second * 30,
				},
			},
			err: fmt.Errorf("exporter has an invalid TCP endpoint: address http://localhost:24224: too many colons in address"),
		},
		{
			desc: "Config is valid",
			cfg: &Config{
				TCPClientSettings: TCPClientSettings{
					Endpoint:          validEndpoint,
					ConnectionTimeout: time.Second * 30,
				},
			},
			err: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
