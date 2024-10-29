// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/axoflow/fluentforwardexporter/internal/metadata"
)

// NewFactory creates a factory for the fluentforward exporter.
func NewFactory() exporter.Factory {
	// later count failed log records
	//_ = view.Register(metricViews()...)

	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TCPClientSettings: TCPClientSettings{
			Endpoint:          "localhost:24224",
			ConnectionTimeout: time.Second * 30,
			TLSSetting: configtls.ClientConfig{
				Insecure:           true,
				InsecureSkipVerify: false,
				Config: configtls.Config{
					CAFile:   "",
					CertFile: "",
					KeyFile:  "",
				},
			},
			SharedKey: "",
		},
		RequireAck:   false,
		Tag:          "tag",
		CompressGzip: false,
		DefaultLabelsEnabled: map[string]bool{
			"time":     true,
			"exporter": true,
			"job":      true,
			"instance": true,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueConfig:   exporterhelper.NewDefaultQueueConfig(),
	}
}

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Logs, error) {
	exporterConfig := config.(*Config)
	exp := newExporter(exporterConfig, set.TelemetrySettings)

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		config,
		exp.pushLogData,
		// explicitly disable since we rely on net.Dialer timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(exporterConfig.BackOffConfig),
		exporterhelper.WithQueue(exporterConfig.QueueConfig),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop),
	)
}
