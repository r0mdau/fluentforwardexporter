// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewExporter(t *testing.T) {
	config := &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint:          validEndpoint,
			ConnectionTimeout: time.Second * 30,
		},
	}

	// Create a new instance of the component.TelemetrySettings struct with a mock logger
	logger := zap.NewNop()
	settings := component.TelemetrySettings{
		Logger: logger,
	}

	exporter := newExporter(config, settings)

	assert.Equal(t, config, exporter.config)
	assert.Equal(t, logger, exporter.settings.Logger)
}
func TestStart(t *testing.T) {
	config := &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint:          validEndpoint,
			ConnectionTimeout: time.Second * 30,
		},
	}

	logger := zap.NewNop()
	settings := component.TelemetrySettings{
		Logger: logger,
	}

	exporter := newExporter(config, settings)

	err := exporter.start(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, exporter.client)
}

func TestStartInvalidEndpointErrorLog(t *testing.T) {

	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	config := &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint:          "invalidEndpoint",
			ConnectionTimeout: time.Second * 30,
		},
	}

	settings := component.TelemetrySettings{
		Logger: observedLogger,
	}

	exporter := newExporter(config, settings)

	err := exporter.start(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, exporter.client)
	require.Equal(t, 2, observedLogs.Len())

	assert.Equal(t, "Creating the Fluent Forward exporter", observedLogs.All()[0].Message)
	assert.Equal(t, "Failed to connect to the endpoint invalidEndpoint", observedLogs.All()[1].Message)
}
