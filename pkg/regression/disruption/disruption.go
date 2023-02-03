package disruption

import "cloud.google.com/go/bigquery"

type RegressionDetector struct {
	BigQueryClient *bigquery.Client
}
