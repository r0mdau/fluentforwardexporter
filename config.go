// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type TCPClientSettings struct {
	// The target endpoint URI to send data to (e.g.: some.url:24224).
	Endpoint string `mapstructure:"endpoint"`

	// Connection Timeout parameter configures `net.Dialer`.
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
}

// Config defines configuration for fluentforward exporter.
type Config struct {
	TCPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// RequireAck enables the acknowledgement feature.
	RequireAck bool `mapstructure:"require_ack"`

	// The Fluent tag parameter used for routing
	Tag string `mapstructure:"tag"`

	// CompressGzip enables gzip compression for the payload.
	CompressGzip bool `mapstructure:"compress_gzip"`

	// DefaultLabelsEnabled is a map of default attributes to be added to each log record.
	DefaultLabelsEnabled map[string]bool `mapstructure:"default_labels_enabled"`

	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`
}

var _ component.Config = (*Config)(nil)

func (config *Config) Validate() error {
	if err := config.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	if _, err := url.Parse(config.Endpoint); config.Endpoint == "" || err != nil {
		return fmt.Errorf("\"endpoint\" must be a valid URL")
	}
	return nil
}
