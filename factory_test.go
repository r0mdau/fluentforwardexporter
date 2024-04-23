// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter

import (
	"context"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	validEndpoint = "localhost:24224"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestNewExporterMinimalConfig(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &Config{
			TCPClientSettings: TCPClientSettings{
				Endpoint:          validEndpoint,
				ConnectionTimeout: time.Second * 30,
			},
		}
		exp := newExporter(config, componenttest.NewNopTelemetrySettings())
		require.NotNil(t, exp)
	})
}

func TestNewExporterFullConfig(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &Config{
			TCPClientSettings: TCPClientSettings{
				Endpoint:          validEndpoint,
				ConnectionTimeout: time.Second * 30,
				TLSSetting: configtls.TLSClientSetting{
					Insecure:           true,
					InsecureSkipVerify: false,
				},
				SharedKey: "otelcol-dev",
			},
			RequireAck:   true,
			Tag:          "tag",
			CompressGzip: true,
			DefaultLabelsEnabled: map[string]bool{
				"time":     true,
				"exporter": true,
				"job":      true,
				"instance": true,
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
		}
		exp := newExporter(config, componenttest.NewNopTelemetrySettings())
		require.NotNil(t, exp)
	})
}

func TestStartAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint:          validEndpoint,
			ConnectionTimeout: time.Second * 30,
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestStopAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint:          validEndpoint,
			ConnectionTimeout: time.Second * 30,
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, exp.stop(context.Background()))
}
