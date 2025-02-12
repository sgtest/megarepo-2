package files

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	exists *observation.Operation
	get    *observation.Operation
}

func newOperations(observationContext *observation.Context) *operations {
	m := metrics.NewREDMetrics(
		observationContext.Registerer,
		"apiworker_apiclient_files",
		metrics.WithLabels("op"),
		metrics.WithCountHelp("Total number of method invocations."),
	)

	op := func(name string) *observation.Operation {
		return observationContext.Operation(observation.Op{
			Name:              fmt.Sprintf("apiworker.apiclient.files.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           m,
		})
	}

	return &operations{
		exists: op("Exists"),
		get:    op("Get"),
	}
}
