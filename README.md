# Fluent Forward Exporter
<!-- status autogenerated section -->
| Status        |           |
| ------------- |-----------|
| Stability     | [development]: logs   |
| Distributions | [contrib] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Aexporter%2Ffluentforward%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Aexporter%2Ffluentforward) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Aexporter%2Ffluentforward%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Aexporter%2Ffluentforward) |
| [Code Owners](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#becoming-a-code-owner)    | [@r0mdau](https://www.github.com/r0mdau) |

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
<!-- end autogenerated section -->

Forward is the protocol used by Fluentd to route message between peers.

- Protocol specification: [Forward protocol specification v1](https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1)
- Library used [IBM/fluent-forward-go](https://github.com/IBM/fluent-forward-go) (MIT License)

## Getting Started

### Settings

| Property | Default value | Type | Description |
|---|---|---|---|
| endpoint.tcp_addr |  | string | **MANDATORY** Target URL to send `Forward` log streams to |
| endpoint.validate_tcp_resolution | false | bool | Controls whether to validate the tcp address and fail at startup. |
| connection_timeout | 30s | time.Duration | Maximum amount of time a dial will wait for a connect to complete |
| tls.insecure | true | bool | If set to **true**, the connexion is not secured with TLS. |
| tls.insecure_skip_verify | false | bool | Controls whether the exporter verifies the server's certificate chain and host name. If **true**, any certificate is accepted and any host name. This mode is susceptible to man-in-the-middle attacks |
| tls.ca_file | "" | string | Used for mTLS. Path to the CA cert. For a client this verifies the server certificate |
| tls.cert_file | "" | string | Used for mTLS. Path to the client TLS cert to use |
| tls.key_file | "" | string | Used for mTLS. Path to the client TLS key to use |
| shared_key | "" | string | A key string known by the server, used for authorization |
| require_ack| false | bool | Protocol delivery acknowledgment for log streams : true = at-least-once, false = at-most-once |
| tag | "tag" | string | Fluentd tag is a string separated by '.'s (e.g. myapp.access), and is used as the directions for Fluentd's internal routing engine |
| compress_gzip | false | bool | Transparent data compression. You can use this feature to reduce the transferred payload size |
| default_labels_enabled | true | map[string]bool | If omitted then default labels will be added. If one of the labels is omitted then this label will be added |

See the default values in the method `createDefaultConfig()` in [factory.go](factory.go) file.

Example, for `default_labels_enabled` that will add only the `timestamp` attribute in the log record:

```yaml
exporters:
  fluentforward:
    endpoint:
      tcp_addr: a.new.fluentforward.target:24224
    connection_timeout: 10s
    require_ack: true
    tag: nginx
    compress_gzip: true
    default_labels_enabled:
      timestamp: true
      level: false
      message: false
```

But a best practice is to have at least `timestamp`, `level` and `message` in the exported log record to a Fluent endpoint.

Example with TLS enabled and shared key:

```yaml
exporters:
  fluentforward:
    endpoint:
      tcp_addr: a.new.fluentforward.target:24224
    connection_timeout: 10s
    tls:
      insecure: false
    shared_key: otelcol-dev
```

Example with mutual TLS authentication (mTLS):

```yaml
exporters:
  fluentforward:
    endpoint:
      tcp_addr: a.new.fluentforward.target:24224
    connection_timeout: 10s
    tls:
      insecure: false
      ca_file: ca.crt.pem
      cert_file: client.crt.pem
      key_file: client.key.pem
```

## Severity

OpenTelemetry uses `record.severity` to track log levels.

## Advanced Configuration

Queued retry capabilities are enabled by default, see the [Exporter Helper queuing and retry settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md) to fine tune them.

Example usage:

```yaml
exporters:
  fluentforward:
    endpoint:
      tcp_addr: a.new.fluentforward.target:24224
    connection_timeout: 10s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 5m
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 2000
```
