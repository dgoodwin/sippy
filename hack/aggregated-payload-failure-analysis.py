#!/usr/bin/env python

import psycopg2
import os
import re

def extract_summary_variables(text):
    # Define the regex pattern to capture the summary portion
    pattern = r'summary: \'Failed: Passed (\d+) times, failed (\d+) times.  \((P\d+=([\d.]+)s)( \(grace=(\d+))?\)? requiredPasses=(\d+)'
    #pattern = r'summary: \'Failed: Passed (\d+) times, failed (\d+) times.  \((P\d+=([\d.]+)s)( \(grace=(\d+))?\)? requiredPasses=(\d+) successes=\[([\d\s=]+)\] failures=\[([\d\s=]+)\]'

    # Search for the pattern in the text
    match = re.search(pattern, text)

    if match:
        # Extract the variables
        successes = int(match.group(1))
        failures = int(match.group(2))
        percentile = match.group(3)  # Contains "P85"
        percentile_value = float(match.group(4))  # Contains "2.00"
        grace = int(match.group(6)) if match.group(6) else None
        required_passes = int(match.group(7))
        #successes_text = match.group(4)
        #failures_text = match.group(5)

        succeses_count = 0
        matches = re.search(r'successes=\[(.*?)\]', text)
        if matches:
            successes_text = matches.group(1)
            successes_count = len(successes_text.split(" "))
        else:
            print("Successes not found in the string.")

        failures_count = 0
        matches = re.search(r'failures=\[(.*?)\]', text)
        total_failure_seconds = 0.0
        over_percentile_failure_seconds = 0.0
        if matches:
            failures_text = matches.group(1)
            failures = failures_text.split(" ") # jobid=Xs, we're after the X
            failures_count = len(failures)
            for fail in failures:
                fail_seconds = float(fail.split("=")[1][:-1]) # split on the =, remove the trailing s, convert to a float
                total_failure_seconds += fail_seconds
                over_percentile_failure_seconds += (fail_seconds - percentile_value) # subtract what was allowed, this is how much we missed by in total
        else:
            print("Failures not found in the string.")
        average_failed_by_seconds = over_percentile_failure_seconds / failures_count

        #successes_count = len(re.findall(r'\d+=', successes_text))
        #failures_count = len(re.findall(r'\d+=', failures_text))

        # Calculate totalFailureSeconds and averageFailureSeconds
        # failures_pattern = r'\[(\d+)=(\d+)s'
        # failures_matches = re.findall(failures_pattern, text)
        # total_failure_seconds = sum(int(match[1]) for match in failures_matches)
        # average_failure_seconds = total_failure_seconds / failures if failures > 0 else 0

        return {
            'percentile': percentile,
            'percentileValue': percentile_value,
            'grace': grace,
            'requiredPasses': required_passes,
            'successes': successes_count,
            'failures': failures_count,
            'totalFailureSeconds': total_failure_seconds,
            'totalFailedBySeconds': over_percentile_failure_seconds,
            'averageFailedBySeconds': average_failed_by_seconds
        }
    else:
        return None

search_counters = {
        'disruption P70 should not be worse': 0,
        'disruption P85 should not be worse': 0,
        'disruption P95 should not be worse': 0,
        #'zero-disruption should not be worse': 0,
        #'plus five standard deviations': 0,
    }

if __name__ == '__main__':
    conn = psycopg2.connect(os.environ['DSN_PROD'])
    print('connected')
    cur = conn.cursor()
    cur.execute("select id, release_tag from release_tags where (release = %s or release = '4.15') and phase = 'Rejected' and release_time >= NOW() - INTERVAL '30 days' order by release_time desc", ('4.14',))
    rows = cur.fetchall()
    print("Found %d failed payloads" % len(rows))

    failed_tests_in_payload_query = '''SELECT DISTINCT
	   rt.release_tag,
       t.name as test_name,
       pj.name as job_name,
       pjr.url as prow_job_run_url,
       pjrt.id,
       pjrto.output
FROM
     release_tags rt,
     release_job_runs rjr,
     prow_job_run_tests pjrt,
     prow_job_run_test_outputs pjrto,
     tests t,
     prow_jobs pj,
     prow_job_runs pjr
WHERE
    rt.release_tag = %s
    AND rjr.kind = 'Blocking'
    AND rjr.release_tag_id = rt.id
    AND rjr.State = 'Failed'
    AND pjrt.prow_job_run_id = rjr.prow_job_run_id
    AND pjrt.status = 12
    AND pjrt.id = pjrto.prow_job_run_test_id
    AND t.id = pjrt.test_id
    AND pjr.id = pjrt.prow_job_run_id
    AND pj.id = pjr.prow_job_id
ORDER BY pjrt.id DESC'''
    only_disrupted_ctr = 0
    some_disrupted_ctr = 0
    test_fail_counters = {}
    for row in rows:
        cur2 = conn.cursor()
        payload = row[1]
        cur2.execute(failed_tests_in_payload_query, (row[1],))
        failed_tests = cur2.fetchall()

        disrupt_test_failures = []
        for ft in failed_tests:
            test_name = ft[1]
            test_output = ft[5]

            for k in search_counters:
                if k in test_name:
                    search_counters[k] += 1
                    print(test_name)
                    keep_printing = False
                    summary_line = ''
                    for line in test_output.split('\n'):
                        if line.startswith('summary:'):
                            summary_line = '%s%s' % (summary_line, line)
                            keep_printing = True
                            continue
                        if keep_printing and line.startswith('passes:'):
                            keep_printing = False
                            continue
                        if keep_printing:
                            summary_line = '%s%s' % (summary_line, line[1:])

                    print(summary_line)

                    print(extract_summary_variables(summary_line))
                    print()
                    print()



