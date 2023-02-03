package disruption

import (
	"context"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"pgregory.net/changepoint"
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
	nurpResults := map[nurp][]disruptionPercentiles{}
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

		// sort into slices by nurp, we already know we're receiving results in order by date
		// due to our query
		if _, ok := nurpResults[p.nurp]; !ok {
			nurpResults[p.nurp] = []disruptionPercentiles{}
		}
		nurpResults[p.nurp] = append(nurpResults[p.nurp], p)

	}
	log.WithField("results", resultsCtr).Info("done processing disruption percentiles")
	log.WithField("nurps", len(nurpResults)).Info("sorted into distinct nurps")

	for k := range nurpResults {
		nlog := log.WithField("nurp", k)
		nlog.Info("scanning for regressions")
		rd.scanForRegressions(nurpResults[k], nlog)
	}

	log.Info("trying for a specific nurp that looks easy")
	rd.scanForRegressions(nurpResults[nurp{
		BackendName:  "kube-api-new-connections",
		Platform:     "vsphere",
		Release:      "4.13",
		FromRelease:  "",
		Architecture: "amd64",
		Network:      "ovn",
		IPMode:       "ipv4",
		Topology:     "ha",
	}], log.WithField("foo", "bar"))

	return nil
}

func (rd *RegressionDetector) scanForRegressions(nurpResults []disruptionPercentiles, nlog log.FieldLogger) {
	// We know our results coming in are already sorted by date.
	// We now need to choose what percentile we're going to look for regressions in. We know P99 is far too
	// volatile to see real changes. Right now we focus on P95, but this may need to be lowered in future.
	//
	// Build up a slice of floats with the percentile we want for analysis:
	floats := make([]float64, len(nurpResults))
	for i := range nurpResults {
		floats[i] = nurpResults[i].P95
	}

	// Determine changepoints for this job, i.e., determine when a job
	// broke (or was fixed).
	//changepoints := make([]string, 0)
	for _, cp := range changepoint.NonParametric(floats, 1) {

		nlog.WithField("date", nurpResults[cp].ReportDate).Info("detected change")
		//key := floats[cp].Period.UTC().Format(formatter)
		//changepoints = append(changepoints, key)
	}
}

type nurp struct {
	BackendName  string
	Platform     string
	Release      string
	FromRelease  string
	Architecture string
	Network      string
	IPMode       string
	Topology     string
}

// disruptionPercentiles is a transient struct for processing results from the bigquery table.
type disruptionPercentiles struct {
	nurp
	ReportDate civil.Date
	JobRuns    int
	P50        float64
	P75        float64
	P95        float64
	P99        float64

	//PRRepo         bigquery.NullString `bigquery:"repo"`
}
