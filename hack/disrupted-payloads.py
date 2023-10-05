#!/usr/bin/env python

import psycopg2
import os

search_strings = [
        'disruption P70 should not be worse',
        'disruption P85 should not be worse',
        'disruption P95 should not be worse',
        ]

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
       pjrt.id
FROM
     release_tags rt,
     release_job_runs rjr,
     prow_job_run_tests pjrt,
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
        non_disrupt_test_failures = [] # track test names that are not disruption
        disrupt_test_failures = []
        test_fails_for_this_payload = {}
        for ft in failed_tests:
            test_name = ft[1]

            if test_name not in test_fails_for_this_payload:
                test_fails_for_this_payload[test_name] = 0
            test_fails_for_this_payload[test_name] += 1

            if "BackendDisruption." not in test_name and \
                    'plus five standard deviations' not in test_name:
                    #'remains available using' not in test_name and \ # per job, not that common
                    #'should be available throughout' not in test_name:
                        non_disrupt_test_failures.append((test_name, ft[2]))
            else:
                disrupt_test_failures.append((test_name, ft[2]))
        if len(disrupt_test_failures) > 0:
            some_disrupted_ctr += 1
            if len(non_disrupt_test_failures) == 0:
                only_disrupted_ctr += 1
                print("%s FAILED ONLY ON AGGREGATED DISRUPTION" % payload)
                for ft in disrupt_test_failures:
                    print("    %s - %s" % (ft[0], ft[1]))

        # Transfer failed tests in this payload to the main list, we're just doing this to only count
        # a test once if it blocked a payload:
        for test_name in test_fails_for_this_payload:
            if test_name not in test_fail_counters:
                test_fail_counters[test_name] = 0
            test_fail_counters[test_name] += 1


    print()
    print("Top failing tests")
    sorted_tests = sorted(test_fail_counters.items(), key=lambda x: x[1], reverse=True)[:20]
    # Print the top 20 test names and their corresponding values
    for test_name, count in sorted_tests:
        print(f"Test Name: {test_name}, Count: {count}")

    print()
    print("%d/%d failed payloads included some disruption" % (some_disrupted_ctr, len(rows)))
    print("%d/%d failed payloads were only due to disruption" % (only_disrupted_ctr, len(rows)))
    print("Estimated cost at $500/payload = %d/month" % (500*only_disrupted_ctr))
