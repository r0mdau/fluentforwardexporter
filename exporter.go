// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	fclient "github.com/IBM/fluent-forward-go/fluent/client"
	"github.com/IBM/fluent-forward-go/fluent/protocol"
	fproto "github.com/IBM/fluent-forward-go/fluent/protocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
)

type fluentforwardExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *fclient.Client
	wg       sync.WaitGroup
}

func newExporter(config *Config, settings component.TelemetrySettings) *fluentforwardExporter {
	settings.Logger.Info("Creating the Fluent Forward exporter")

	return &fluentforwardExporter{
		config:   config,
		settings: settings,
	}
}

func (f *fluentforwardExporter) start(_ context.Context, host component.Host) error {
	connFactory := &fclient.ConnFactory{
		Address: f.config.Endpoint,
		Timeout: f.config.ConnectionTimeout,
	}
	if f.config.TLSSetting.Enabled {
		connFactory.TLSConfig = &tls.Config{
			InsecureSkipVerify: f.config.TLSSetting.InsecureSkipVerify,
		}
	}

	client := fclient.New(fclient.ConnectionOptions{
		Factory:    connFactory,
		RequireAck: f.config.RequireAck,
	})

	if err := client.Connect(); err != nil {
		f.settings.Logger.Error(fmt.Sprintf("The fluentforward exporter failed to connect to its endpoint %s when starting", f.config.Endpoint))
	}

	f.client = client

	return nil
}

func (f *fluentforwardExporter) stop(context.Context) (err error) {
	f.wg.Wait()
	return f.client.Disconnect()
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
					Timestamp: fproto.EventTimeNow(),
					Record:    f.convertLogToMap(log),
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
	m["severity"] = lr.SeverityText()
	m["message"] = lr.Body().AsString()
	for key, val := range f.config.DefaultLabelsEnabled {
		if val {
			attribute, found := lr.Attributes().Get(key)
			if found {
				m[key] = attribute.AsString()
			}
		}
	}
	return m
}

type sendFunc func(string, protocol.EntryList) error

func (f *fluentforwardExporter) send(sendMethod sendFunc, entries []fproto.EntryExt) error {
	err := sendMethod(f.config.Tag, entries)
	if err != nil {
		if errr := f.client.Reconnect(); errr != nil {
			return errr
		}
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
