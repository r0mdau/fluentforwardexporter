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

type TCPClientSettings struct {
	// The target endpoint URI to send data to (e.g.: some.url:24224).
	Endpoint string `mapstructure:"endpoint"`

	// Connection Timeout parameter configures `net.Dialer`.
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.ClientConfig `mapstructure:"tls"`

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

	// KubernetesMetadata includes kubernetes metadata as a nested object.
	// It leverages resources attributes provided by k8sattributesprocessor
	//
	// Configuration example
	// ```
	// kubernetes_metadata:
	//   key: kubernetes
	//   include_pod_labels: true
	// ```
	//
	// Resulting record structure:
	// ```
	// kubernetes:
	//   namespace_name: default
	//   container_name: nginx
	//   pod_name: nginx-59f678c4b-p6lk6
	//   labels:
	//     app.kubernetes.io/name: nginx
	//   host: gke-dev-node-pool-8-cf541dd4-98ro
	// ```
	//
	KubernetesMetadata *KubernetesMetadata `mapstructure:"kubernetes_metadata,omitempty"`

	exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig  `mapstructure:"retry_on_failure"`
}

type KubernetesMetadata struct {
	Key              string `mapstructure:"key"`
	IncludePodLabels bool   `mapstructure:"include_pod_labels"`
}

var _ component.Config = (*Config)(nil)

func (config *Config) Validate() error {
	if err := config.QueueConfig.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	// Resolve TCP address just to ensure that it is a valid one. It is better
	// to fail here than at when the exporter is started.
	if _, err := net.ResolveTCPAddr("tcp", config.Endpoint); err != nil {
		return fmt.Errorf("exporter has an invalid TCP endpoint: %w", err)
	}
	return nil
}
