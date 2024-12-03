// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"fmt"
	"strings"
	"sync"

	fclient "github.com/IBM/fluent-forward-go/fluent/client"
	"github.com/IBM/fluent-forward-go/fluent/protocol"
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
		RequireAck: f.config.RequireAck,
	}

	tlsConfig, err := f.config.ClientConfig.LoadTLSConfig(ctx)
	if err != nil {
		return err
	}
	connFactory := &fclient.ConnFactory{
		Address:   f.config.Endpoint,
		Timeout:   f.config.ConnectionTimeout,
		TLSConfig: tlsConfig,
	}

	connOptions.Factory = connFactory

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
	return f.client.Disconnect()
}

// connectForward connects to the Fluent Forward endpoint and keep running otel even if the connection is failing
func (f *fluentforwardExporter) connectForward() {
	if err := f.client.Connect(); err != nil {
		f.settings.Logger.Error(fmt.Sprintf("Failed to connect to the endpoint %s", f.config.Endpoint))
		return
	}
	f.settings.Logger.Info(fmt.Sprintf("Successfull connection to the endpoint %s", f.config.Endpoint))

	if f.config.SharedKey != "" {
		if err := f.client.Handshake(); err != nil {
			f.settings.Logger.Error(fmt.Sprintf("Failed shared key handshake with the endpoint %s", f.config.Endpoint))
			return
		}
		f.settings.Logger.Info("Successfull shared key handshake with the endpoint")
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
					Timestamp: fproto.EventTimeNow(),
					Record:    f.convertLogToMap(log, rls.At(i)),
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

func (f *fluentforwardExporter) convertLogToMap(lr plog.LogRecord, res plog.ResourceLogs) map[string]interface{} {
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
	if f.config.KubernetesMetadata != nil {
		key := f.config.KubernetesMetadata.Key
		if f.config.KubernetesMetadata.Key == "" {
			key = "kubernetes"
		}
		var namespace, container, pod, node string
		var labels map[string]string
		res.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if k == "k8s.namespace.name" {
				namespace = v.AsString()
				return true
			}
			if k == "k8s.container.name" {
				container = v.AsString()
			}
			if k == "k8s.pod.name" {
				pod = v.AsString()
			}
			if k == "k8s.node.name" {
				node = v.AsString()
			}
			if f.config.KubernetesMetadata.IncludePodLabels && strings.HasPrefix(k, "k8s.pod.labels.") {
				if labels == nil {
					labels = make(map[string]string)
				}
				labelKey := strings.TrimPrefix(k, "k8s.pod.labels.")
				labels[labelKey] = v.AsString()
			}
			return true
		})

		k8sMetadata := map[string]interface{}{
			"namespace_name": namespace,
			"container_name": container,
			"pod_name":       pod,
			"host":           node,
		}

		if f.config.KubernetesMetadata.IncludePodLabels {
			k8sMetadata["labels"] = labels
		}

		m[key] = k8sMetadata
	}

	f.settings.Logger.Debug(fmt.Sprintf("message %+v", m))

	return m
}

type sendFunc func(string, protocol.EntryList) error

func (f *fluentforwardExporter) send(sendMethod sendFunc, entries []fproto.EntryExt) error {
	err := sendMethod(f.config.Tag, entries)
	// sometimes the connection is lost, we try to reconnect and send the data again
	if err != nil {
		if errr := f.client.Disconnect(); errr != nil {
			return errr
		}
		f.settings.Logger.Warn(fmt.Sprintf("Failed to send data to the endpoint %s, trying to reconnect", f.config.Endpoint))
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
