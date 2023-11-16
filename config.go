// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type TCPClientSettings struct {
	// The target URL to send data to (e.g.: http://some.url:9411/v1/traces).
	Endpoint string `mapstructure:"endpoint"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls"`
}

// Config defines configuration for fluentforward exporter.
type Config struct {
	TCPClientSettings            `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	DefaultLabelsEnabled map[string]bool `mapstructure:"default_labels_enabled"`
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
