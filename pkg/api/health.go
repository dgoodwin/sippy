package api

import (
	"net/http"
	"time"

	sippyv1 "github.com/openshift/sippy/pkg/apis/sippy/v1"
	sippyprocessingv1 "github.com/openshift/sippy/pkg/apis/sippyprocessing/v1"
)

// PrintOverallReleaseHealth gives a summarized status of the overall health, including
// infrastructure, install, upgrade, and variant success rates.
func PrintOverallReleaseHealth(w http.ResponseWriter, curr, twoDay, prev sippyprocessingv1.TestReport) {
	type indicator struct {
		Current  sippyv1.PassRate `json:"current"`
		Previous sippyv1.PassRate `json:"previous"`
	}
	indicators := make(map[string]indicator)

	// Infrastructure
	res := curr.TopLevelIndicators.Infrastructure.TestResultAcrossAllJobs
	passPercent := res.PassPercentage
	total := res.Successes + res.Failures + res.Flakes
	currentPassRate := sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	res = prev.TopLevelIndicators.Infrastructure.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	previousPassRate := sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	indicators["infrastructure"] = indicator{
		Current:  currentPassRate,
		Previous: previousPassRate,
	}

	// Install
	res = curr.TopLevelIndicators.Install.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	currentPassRate = sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	res = prev.TopLevelIndicators.Install.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	previousPassRate = sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	indicators["install"] = indicator{
		Current:  currentPassRate,
		Previous: previousPassRate,
	}

	// Upgrade
	res = curr.TopLevelIndicators.Upgrade.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	currentPassRate = sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	res = prev.TopLevelIndicators.Upgrade.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	previousPassRate = sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	indicators["upgrade"] = indicator{
		Current:  currentPassRate,
		Previous: previousPassRate,
	}

	// Tests
	res = curr.TopLevelIndicators.Tests.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	currentPassRate = sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	res = prev.TopLevelIndicators.Tests.TestResultAcrossAllJobs
	passPercent = res.PassPercentage
	total = res.Successes + res.Failures + res.Flakes
	previousPassRate = sippyv1.PassRate{
		Percentage: passPercent,
		Runs:       total,
	}

	indicators["tests"] = indicator{
		Current:  currentPassRate,
		Previous: previousPassRate,
	}

	type variants struct {
		Current  sippyprocessingv1.VariantHealth `json:"current"`
		Previous sippyprocessingv1.VariantHealth `json:"previous"`
	}

	type health struct {
		Indicators  map[string]indicator         `json:"indicators"`
		Variants    variants                     `json:"variants"`
		LastUpdated time.Time                    `json:"last_updated"`
		Promotions  map[string]time.Time         `json:"promotions"`
		Warnings    []string                     `json:"warnings"`
		Current     sippyprocessingv1.Statistics `json:"current_statistics"`
		TwoDay      sippyprocessingv1.Statistics `json:"two_day_statistics"`
		Previous    sippyprocessingv1.Statistics `json:"previous_statistics"`
	}

	RespondWithJSON(http.StatusOK, w, health{
		Indicators:  indicators,
		LastUpdated: curr.Timestamp,
		Variants: variants{
			Current:  curr.TopLevelIndicators.Variant,
			Previous: prev.TopLevelIndicators.Variant,
		},
		Current:  curr.JobStatistics,
		TwoDay:   twoDay.JobStatistics,
		Previous: prev.JobStatistics,
		Warnings: append(curr.AnalysisWarnings, prev.AnalysisWarnings...),
	})
}
