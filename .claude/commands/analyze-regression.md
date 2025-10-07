Analyze a CI test regression ID and summarize it's characteristics for the user.

Follow these steps:
- Run "scripts/fetch-regression-details.py REGRESSION_ID" to obtain details about this test regression.
- Summarize the regression status:
  - When it was opened
  - When it was closed, of it is an on-going regression
  - Explain the status code
- Check if the BaseRelease appears to be more than one minor version ahead of the sample release. This would indicate we found a better pass rate in earlier releases, so we compared against that release instead of the one prior to prevent a test gradually getting worse.
- Look for patterns in the failed sample job runs:
  - If the failed job runs appear to mostly have mass test failures (more than 10), this could indicate the test is not at fault and we're experiencing catastrophic cluster failure which is affecting this test.
  - If the test fails the job by itself most of the time, this indicates a more legitimate problem just with this test.
  - If the sample states show failures but few or no flakes, and the base stats show mostly flakes but few or no failures, inform the user this test may have had it's ability to flake removed and this is why it is now showing up. Compare the base flake rate to the sample fail rate so the user can see if we are now failing at a similar rate we used to flake. 
