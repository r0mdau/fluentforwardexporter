// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	fclient "github.com/IBM/fluent-forward-go/fluent/client"
	fproto "github.com/IBM/fluent-forward-go/fluent/protocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type fluentforwardExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *fclient.Client
	wg       sync.WaitGroup
	// sendMu serializes sends: the client does not guard concurrent writes
	// on the connection when RequireAck is false, so parallel queue consumers
	// would interleave their frames on the wire.
	sendMu sync.Mutex
}

func newExporter(config *Config, settings component.TelemetrySettings) *fluentforwardExporter {
	settings.Logger.Info("Creating the Fluent Forward exporter")

	return &fluentforwardExporter{
		config:   config,
		settings: settings,
	}
}

func (f *fluentforwardExporter) start(ctx context.Context, host component.Host) error {
	connOptions := fclient.ConnectionOptions{
		RequireAck:        f.config.RequireAck,
		ConnectionTimeout: f.config.ConnectionTimeout,
	}

	tlsConfig, err := f.config.ClientConfig.LoadTLSConfig(ctx)
	if err != nil {
		return err
	}
	connFactory := &fclient.ConnFactory{
		Address:   f.config.Endpoint.TCPAddr,
		Timeout:   f.config.ConnectionTimeout,
		TLSConfig: tlsConfig,
	}

	connOptions.Factory = &deadlineConnFactory{
		ConnectionFactory: connFactory,
		timeout:           f.config.ConnectionTimeout,
	}

	if f.config.SharedKey != "" {
		connOptions.AuthInfo = fclient.AuthInfo{
			SharedKey: []byte(f.config.SharedKey),
		}
	}

	client := fclient.New(connOptions)
	f.client = client
	f.connectForward()

	return nil
}

func (f *fluentforwardExporter) stop(context.Context) (err error) {
	f.wg.Wait()
	// client is nil when start was never called or failed before creating it.
	if f.client == nil {
		return nil
	}
	return f.client.Disconnect()
}

// connectForward connects to the Fluent Forward endpoint and keep running otel even if the connection is failing
func (f *fluentforwardExporter) connectForward() {
	if err := f.client.Connect(); err != nil {
		f.settings.Logger.Error(fmt.Sprintf("Failed to connect to the endpoint %s", f.config.Endpoint.TCPAddr))
		return
	}
	f.settings.Logger.Info(fmt.Sprintf("Successful connection to the endpoint %s", f.config.Endpoint.TCPAddr))

	if f.config.SharedKey != "" {
		if err := f.client.Handshake(); err != nil {
			f.settings.Logger.Error(fmt.Sprintf("Failed shared key handshake with the endpoint %s", f.config.Endpoint.TCPAddr))
			return
		}
		f.settings.Logger.Info("Successful shared key handshake with the endpoint")
	}
}

func (f *fluentforwardExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	// move for loops into a translator
	entries := []fproto.EntryExt{}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				entry := fproto.EntryExt{
					Timestamp: fproto.EventTime{
						Time: logTimestamp(log).AsTime(),
					},
					Record: f.convertLogToMap(log),
				}
				entries = append(entries, entry)
			}
		}
	}

	if f.config.CompressGzip {
		return f.sendCompressed(entries)
	}
	return f.sendForward(entries)
}

func (f *fluentforwardExporter) convertLogToMap(lr plog.LogRecord) map[string]interface{} {
	// move function into a translator
	m := make(map[string]interface{})
	for k, v := range f.config.DefaultLabelsEnabled {
		if v {
			switch k {
			case "level":
				m[k] = lr.SeverityText()
			case "message":
				m[k] = lr.Body().AsString()
			case "timestamp":
				m[k] = logTimestamp(lr).AsTime().UTC().Format(time.RFC3339Nano)
			}
		}
	}

	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		m[k] = v.AsString()
		return true
	})
	return m
}

// logTimestamp returns the time the event occurred, falling back to the time
// it was observed: many receivers only set ObservedTimestamp, and an unset
// Timestamp would otherwise be sent as the Unix epoch.
func logTimestamp(lr plog.LogRecord) pcommon.Timestamp {
	if ts := lr.Timestamp(); ts != 0 {
		return ts
	}
	return lr.ObservedTimestamp()
}

type sendFunc func(string, fproto.EntryList) error

func (f *fluentforwardExporter) send(sendMethod sendFunc, entries []fproto.EntryExt) error {
	f.sendMu.Lock()
	defer f.sendMu.Unlock()

	err := sendMethod(f.config.Tag, entries)
	// sometimes the connection is lost, we try to reconnect and send the data again
	if err != nil {
		if errr := f.client.Disconnect(); errr != nil {
			return errr
		}
		f.settings.Logger.Warn(fmt.Sprintf("Failed to send data to the endpoint %s, trying to reconnect", f.config.Endpoint.TCPAddr))
		f.connectForward()
		err = sendMethod(f.config.Tag, entries)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *fluentforwardExporter) sendCompressed(entries []fproto.EntryExt) error {
	return f.send(f.client.SendCompressed, entries)
}

func (f *fluentforwardExporter) sendForward(entries []fproto.EntryExt) error {
	return f.send(f.client.SendForward, entries)
}

// deadlineConnFactory wraps a ConnectionFactory so that reads and writes on
// its connections time out instead of blocking forever on an unresponsive
// peer. The client sets no deadlines of its own on handshakes or writes.
type deadlineConnFactory struct {
	fclient.ConnectionFactory
	timeout time.Duration
}

func (f *deadlineConnFactory) New() (net.Conn, error) {
	conn, err := f.ConnectionFactory.New()
	if err != nil || f.timeout <= 0 {
		return conn, err
	}
	return &deadlineConn{Conn: conn, timeout: f.timeout}, nil
}

// deadlineConn refreshes the deadline before every Read and Write, so a peer
// that keeps making progress is never cut off, while a stalled one is.
type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c *deadlineConn) Read(b []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func (c *deadlineConn) Write(b []byte) (int, error) {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}
