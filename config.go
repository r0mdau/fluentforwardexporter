// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// TCPClientSettings defines common settings for a TCP client.
type TCPClientSettings struct {
	// Endpoint to send logs to.
	Endpoint `mapstructure:"endpoint"`

	// Connection Timeout parameter configures `net.Dialer`.
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`

	// ClientConfig struct exposes TLS client configuration.
	ClientConfig configtls.ClientConfig `mapstructure:"tls"`

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

	QueueBatchConfig          exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

// Endpoint defines the address of the server to connect to.
type Endpoint struct {
	// TCPAddr is the address of the server to connect to.
	TCPAddr string `mapstructure:"tcp_addr"`
	// Controls whether to validate the tcp address.
	ValidateTCPResolution bool `mapstructure:"validate_tcp_resolution"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the configuration is valid
func (config *Config) Validate() error {
	if err := config.QueueBatchConfig.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	if config.TCPClientSettings.Endpoint.ValidateTCPResolution {
		// Resolve TCP address just to ensure that it is a valid one. It is better
		// to fail here than at when the exporter is started.
		if _, err := net.ResolveTCPAddr("tcp", config.Endpoint.TCPAddr); err != nil {
			return fmt.Errorf("exporter has an invalid TCP endpoint: %w", err)
		}
	}

	return nil
}
