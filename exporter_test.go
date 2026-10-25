// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	fproto "github.com/IBM/fluent-forward-go/fluent/protocol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewExporter(t *testing.T) {
	config := &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint: Endpoint{
				TCPAddr: validEndpoint,
			},
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
			Endpoint: Endpoint{
				TCPAddr: validEndpoint,
			},
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
			Endpoint: Endpoint{
				TCPAddr:               "invalidEndpoint",
				ValidateTCPResolution: true,
			},
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

func TestPushLogDataConcurrentSendsDoNotInterleave(t *testing.T) {
	for _, compress := range []bool{false, true} {
		t.Run(fmt.Sprintf("compress=%v", compress), func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer func() { _ = ln.Close() }()

			const goroutines, perGoroutine = 10, 20
			decodeErr := make(chan error, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					decodeErr <- err
					return
				}
				defer func() { _ = conn.Close() }()
				r := msgp.NewReader(conn)
				for i := 0; i < goroutines*perGoroutine; i++ {
					var err error
					if compress {
						var m fproto.PackedForwardMessage
						err = m.DecodeMsg(r)
					} else {
						var m fproto.ForwardMessage
						err = m.DecodeMsg(r)
					}
					if err != nil {
						decodeErr <- err
						return
					}
				}
				decodeErr <- nil
			}()

			config := createDefaultConfig().(*Config)
			config.Endpoint.TCPAddr = ln.Addr().String()
			config.CompressGzip = compress
			exporter := newExporter(config, component.TelemetrySettings{Logger: zap.NewNop()})
			require.NoError(t, exporter.start(context.Background(), nil))
			defer func() { assert.NoError(t, exporter.stop(context.Background())) }()

			// Bodies larger than the msgp writer buffer force several writes per frame.
			body := strings.Repeat("x", 8000)
			var wg sync.WaitGroup
			for g := 0; g < goroutines; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < perGoroutine; i++ {
						ld := plog.NewLogs()
						lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
						lr.Body().SetStr(body)
						assert.NoError(t, exporter.pushLogData(context.Background(), ld))
					}
				}()
			}
			wg.Wait()

			select {
			case err := <-decodeErr:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("server did not receive all messages")
			}
		})
	}
}
