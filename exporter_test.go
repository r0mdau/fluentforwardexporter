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
	"go.opentelemetry.io/collector/pdata/pcommon"
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

func TestStopWithoutStart(t *testing.T) {
	exporter := newExporter(createDefaultConfig().(*Config), component.TelemetrySettings{Logger: zap.NewNop()})

	assert.NoError(t, exporter.stop(context.Background()))
}

func TestStopAfterFailedStart(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.ClientConfig.Insecure = false
	config.ClientConfig.CAFile = "does-not-exist.crt"
	exporter := newExporter(config, component.TelemetrySettings{Logger: zap.NewNop()})

	require.Error(t, exporter.start(context.Background(), nil))
	assert.Nil(t, exporter.client)
	assert.NoError(t, exporter.stop(context.Background()))
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

func TestStartHandshakeTimesOutWhenServerIsSilent(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	// Accept connections but never send the HELO the handshake waits for.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	config := createDefaultConfig().(*Config)
	config.Endpoint.TCPAddr = ln.Addr().String()
	config.ConnectionTimeout = 200 * time.Millisecond
	config.SharedKey = "otelcol-dev"
	exporter := newExporter(config, component.TelemetrySettings{Logger: zap.New(observedZapCore)})

	done := make(chan error, 1)
	go func() { done <- exporter.start(context.Background(), nil) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("start blocked on the handshake")
	}
	assert.Equal(t, 1, observedLogs.FilterMessageSnippet("Failed shared key handshake").Len())
	assert.NoError(t, exporter.stop(context.Background()))
}

func TestPushLogDataTimesOutWhenServerStopsReading(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	// Accept connections but never read, so the socket buffers fill up.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	config := createDefaultConfig().(*Config)
	config.Endpoint.TCPAddr = ln.Addr().String()
	config.ConnectionTimeout = 200 * time.Millisecond
	exporter := newExporter(config, component.TelemetrySettings{Logger: zap.NewNop()})
	require.NoError(t, exporter.start(context.Background(), nil))
	defer func() { _ = exporter.stop(context.Background()) }()

	body := strings.Repeat("x", 4<<20)
	done := make(chan error, 1)
	go func() {
		// Keep pushing until the full buffers make a write fail.
		for i := 0; i < 50; i++ {
			ld := plog.NewLogs()
			lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			lr.Body().SetStr(body)
			if err := exporter.pushLogData(context.Background(), ld); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		var netErr net.Error
		require.ErrorAs(t, err, &netErr)
		assert.True(t, netErr.Timeout())
	case <-time.After(20 * time.Second):
		t.Fatal("pushLogData blocked on a stalled server")
	}
}

func TestPushLogDataFallsBackToObservedTimestamp(t *testing.T) {
	eventTime := time.Date(2026, 10, 25, 1, 2, 3, 4000, time.UTC)
	observedTime := time.Date(2026, 10, 25, 5, 6, 7, 8000, time.UTC)

	tests := []struct {
		name      string
		timestamp time.Time
		observed  time.Time
		expected  time.Time
	}{
		{name: "timestamp set", timestamp: eventTime, observed: observedTime, expected: eventTime},
		{name: "only observed timestamp set", observed: observedTime, expected: observedTime},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer func() { _ = ln.Close() }()

			received := make(chan fproto.ForwardMessage, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				defer func() { _ = conn.Close() }()
				var m fproto.ForwardMessage
				if err := m.DecodeMsg(msgp.NewReader(conn)); err == nil {
					received <- m
				}
			}()

			config := createDefaultConfig().(*Config)
			config.Endpoint.TCPAddr = ln.Addr().String()
			exporter := newExporter(config, component.TelemetrySettings{Logger: zap.NewNop()})
			require.NoError(t, exporter.start(context.Background(), nil))
			defer func() { assert.NoError(t, exporter.stop(context.Background())) }()

			ld := plog.NewLogs()
			lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			if !tt.timestamp.IsZero() {
				lr.SetTimestamp(pcommon.NewTimestampFromTime(tt.timestamp))
			}
			lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(tt.observed))
			require.NoError(t, exporter.pushLogData(context.Background(), ld))

			select {
			case m := <-received:
				require.Len(t, m.Entries, 1)
				entry := m.Entries[0]
				assert.True(t, tt.expected.Equal(entry.Timestamp.Time), "event time: got %s, want %s", entry.Timestamp.Time, tt.expected)
				assert.Equal(t, tt.expected.Format(time.RFC3339Nano), entry.Record.(map[string]any)["timestamp"])
			case <-time.After(5 * time.Second):
				t.Fatal("server did not receive the message")
			}
		})
	}
}

func TestPushLogDataKeepsAttributeTypes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	received := make(chan fproto.ForwardMessage, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		var m fproto.ForwardMessage
		if err := m.DecodeMsg(msgp.NewReader(conn)); err == nil {
			received <- m
		}
	}()

	config := createDefaultConfig().(*Config)
	config.Endpoint.TCPAddr = ln.Addr().String()
	exporter := newExporter(config, component.TelemetrySettings{Logger: zap.NewNop()})
	require.NoError(t, exporter.start(context.Background(), nil))
	defer func() { assert.NoError(t, exporter.stop(context.Background())) }()

	ld := plog.NewLogs()
	attrs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes()
	attrs.PutStr("str", "value")
	attrs.PutInt("int", 42)
	attrs.PutDouble("double", 1.5)
	attrs.PutBool("bool", true)
	attrs.PutEmptyBytes("bytes").FromRaw([]byte{0x01, 0x02})
	require.NoError(t, attrs.PutEmptySlice("slice").FromRaw([]any{"a", int64(1)}))
	nested := attrs.PutEmptyMap("map")
	nested.PutStr("key", "nested")
	nested.PutInt("n", 7)
	attrs.PutEmpty("empty")
	require.NoError(t, exporter.pushLogData(context.Background(), ld))

	select {
	case m := <-received:
		require.Len(t, m.Entries, 1)
		record := m.Entries[0].Record.(map[string]any)
		assert.Equal(t, "value", record["str"])
		assert.Equal(t, int64(42), record["int"])
		assert.Equal(t, 1.5, record["double"])
		assert.Equal(t, true, record["bool"])
		assert.Equal(t, []byte{0x01, 0x02}, record["bytes"])
		assert.Equal(t, []any{"a", int64(1)}, record["slice"])
		assert.Equal(t, map[string]any{"key": "nested", "n": int64(7)}, record["map"])
		assert.Contains(t, record, "empty")
		assert.Nil(t, record["empty"])
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive the message")
	}
}
