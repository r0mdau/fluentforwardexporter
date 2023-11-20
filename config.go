// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// TLSClientSetting contains TLS configurations that are specific to client
// connections in addition to the common configurations. This should be used by
// components configuring TLS client connections.
type TLSClientSetting struct {
	// Enabled defines if TLS is enabled or not.
	Enabled bool `mapstructure:"enabled"`

	// InsecureSkipVerify will enable TLS but not verify the certificate.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

type TCPClientSettings struct {
	// The target endpoint URI to send data to (e.g.: some.url:24224).
	Endpoint string `mapstructure:"endpoint"`

	// Connection Timeout parameter configures `net.Dialer`.
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting TLSClientSetting `mapstructure:"tls"`

	// SharedKey is used for authorization with the server that knows it.
	SharedKey string `mapstructure:"shared_key"`
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

	// Resolve TCP address just to ensure that it is a valid one. It is better
	// to fail here than at when the exporter is started.
	if _, err := net.ResolveTCPAddr("tcp", config.Endpoint); err != nil {
		return fmt.Errorf("exporter has an invalid TCP endpoint: %w", err)
	}
	return nil
}
