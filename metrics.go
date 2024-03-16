// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardexporter // import "github.com/r0mdau/fluentforwardexporter"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	lokiExporterFailedToSendLogRecordsDueToMissingLabels = stats.Int64("fluentforwardexporter_send_failed_due_to_missing_labels", "Number of log records failed to send because labels were missing", stats.UnitDimensionless)
)

func metricViews() []*view.View {
	return []*view.View{
		{
			Name:        "fluentforwardexporter_send_failed_due_to_missing_labels",
			Description: "Number of log records failed to send because labels were missing",
			Measure:     lokiExporterFailedToSendLogRecordsDueToMissingLabels,
			Aggregation: view.Count(),
		},
	}
}
