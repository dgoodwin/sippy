package disruption

import (
	"context"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

type RegressionDetector struct {
	BigQueryClient *bigquery.Client
}

func (rd *RegressionDetector) Scan() error {
	log.Info("scanning for disruption regressions")

	query := rd.BigQueryClient.Query(`SELECT * ` +
		"FROM `openshift-ci-data-analysis.ci_data.BackendDisruptionPercentilesByDate` " +
		`WHERE ReportDate >= DATE_SUB(CURRENT_DATE(), INTERVAL 30 DAY)
			AND JobRuns > 100
		ORDER BY ReportDate`)
	// TODO: limit to a certain number of JobRuns?
	/*
		query.Parameters = []bigquery.QueryParameter{
			{
				Name:  "queryFrom",
				Value: lastProwJobRun,
			},
		}
	*/
	it, err := query.Read(context.Background())
	if err != nil {
		log.WithError(err).Error("error querying jobs from bigquery")
		return err
	}

	//prowJobs := []prow.ProwJob{}
	resultsCtr := 0
	for {
		p := disruptionPercentiles{}
		err := it.Next(&p)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.WithError(err).Error("error parsing disruption percentiles")
			return err
		}
		resultsCtr++

		log.Infof("got percentile: %+v", p)

	}
	log.WithField("results", resultsCtr).Info("done processing disruption percentiles")
	return nil
}

// disruptionPercentiles is a transient struct for processing results from the bigquery table.
type disruptionPercentiles struct {
	ReportDate   civil.Date
	BackendName  string
	Platform     string
	Release      string
	FromRelease  string
	Architecture string
	Network      string
	IPMode       string
	Topology     string
	JobRuns      int
	P50          float64
	P75          float64
	P95          float64
	P99          float64

	//PRRepo         bigquery.NullString `bigquery:"repo"`
}
