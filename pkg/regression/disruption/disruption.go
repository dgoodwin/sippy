package disruption

import (
	"context"
	"fmt"

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
		`WHERE ReportDate >= DATE_SUB(CURRENT_DATE(), INTERVAL 60 DAY)
			AND JobRuns > 100
		ORDER BY ReportDate`)
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

	/*
		for k := range nurpResults {
			nlog := log.WithField("nurp", k)
			nlog.Info("scanning for regressions")
			rd.scanForRegressions(nurpResults[k], nlog)
		}
	*/

	// this nurp works well as it's generally 0 with a couple small spikes up to around 1s:
	testNURP := nurp{
		BackendName:  "kube-api-new-connections",
		Platform:     "aws",
		Release:      "4.13",
		FromRelease:  "4.13",
		Architecture: "amd64",
		Network:      "ovn",
		IPMode:       "ipv4",
		Topology:     "ha",
	}
	rd.scanForRegressions(nurpResults[testNURP], log.WithField("nurp", testNURP))

	// works poorly, we were high on the start date, picks up no changes up or down, seems very focused on the first
	// item in your array. could we start on a date where the value was close to the minimum? or average? find a "good" date?
	testNURP = nurp{
		BackendName:  "image-registry-new-connections",
		Platform:     "aws",
		Release:      "4.13",
		FromRelease:  "4.13",
		Architecture: "amd64",
		Network:      "ovn",
		IPMode:       "ipv4",
		Topology:     "ha",
	}
	rd.scanForRegressions(nurpResults[testNURP], log.WithField("nurp", testNURP))

	// also poor. start date was at 14s, fails to see the drop to <2s, fails to see another increase back to 17, fails to 30 on Jan 26, or a drop back to 22
	testNURP = nurp{
		BackendName:  "ingress-to-console-new-connections",
		Platform:     "azure",
		Release:      "4.13",
		FromRelease:  "4.13",
		Architecture: "amd64",
		Network:      "ovn",
		IPMode:       "ipv4",
		Topology:     "ha",
	}
	rd.scanForRegressions(nurpResults[testNURP], log.WithField("nurp", testNURP))

	return nil
}

func (rd *RegressionDetector) scanForRegressions(nurpResults []disruptionPercentiles, nlog log.FieldLogger) {

	nlog.Info("scanning nurp for regressions")

	// hacky attempt: find the minimum value and the date it occurred, we'll start there as the code seems
	// to heavily weight the starting day.
	// Even this doesn't work, the changes picked up are too small still.
	lowestValSeen := nurpResults[0].P95
	lowestValIndex := 0
	for i, nr := range nurpResults {
		if nr.P95 < lowestValSeen {
			lowestValIndex = i
			lowestValSeen = nr.P95
		}
	}
	scanNURPResults := nurpResults[lowestValIndex:]
	nlog.Infof("lowest value %.2f was seen on %s, scanning %d results", lowestValSeen,
		nurpResults[lowestValIndex].ReportDate, len(scanNURPResults))

	// We know our results coming in are already sorted by date.
	// We now need to choose what percentile we're going to look for regressions in. We know P99 is far too
	// volatile to see real changes. Right now we focus on P95, but this may need to be lowered in future.
	//
	// Build up a slice of floats with the percentile we want for analysis:
	floats := make([]float64, len(scanNURPResults))
	for i := range scanNURPResults {
		floats[i] = scanNURPResults[i].P95
	}

	// minSegment here looks like it's the number of data points we require to stay high/low to consider it a change
	// disruption can come and go, so we may not want to use a segment of 1 day as that could be too volatile. we want
	// to detect disruption going up, and staying there.
	for _, cp := range changepoint.NonParametric(floats, 3) {

		changeDateVal := floats[cp]
		prevDateVal := floats[cp-1]
		result := fmt.Sprintf("up from %.2f -> %.2f", prevDateVal, changeDateVal)
		if changeDateVal < prevDateVal {
			result = fmt.Sprintf("down from %.2f -> %.2f", prevDateVal, changeDateVal)
		}
		nlog.WithField("date", scanNURPResults[cp].ReportDate).Infof("detected change: %s", result)
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
