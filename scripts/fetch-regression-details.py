#!/usr/bin/env python3

import argparse
import json
import sys
import urllib.request
from datetime import datetime


def format_date_range(stats):
    """Format date range and calculate duration from stats dict"""
    if 'Start' not in stats or 'End' not in stats:
        return ""
    
    try:
        # Parse ISO format dates (e.g., "2025-09-23T00:00:00Z")
        start_date = datetime.fromisoformat(stats['Start'].replace('Z', '+00:00'))
        end_date = datetime.fromisoformat(stats['End'].replace('Z', '+00:00'))
        
        # Calculate duration in days
        duration = (end_date - start_date).days
        
        # Format as readable date range
        start_str = start_date.strftime('%Y-%m-%d')
        end_str = end_date.strftime('%Y-%m-%d')
        
        return f" ({start_str} to {end_str}, {duration} days)"
    except (ValueError, TypeError):
        return ""


def get_status_description(status_code):
    """Map status code to description based on crtest/types.go constants"""
    status_map = {
        -1000: "Failed Fixed Regression (claimed fix but failures persist)",
        -500: "Extreme Regression (>15% pass rate change)",
        -400: "Significant Regression",
        -300: "Extreme Triaged Regression (clears when triaged incidents factored in)",
        -200: "Significant Triaged Regression (clears when triaged incidents factored in)",
        -150: "Fixed Regression (claimed fixed but not yet rolled off sample window)",
        -100: "Missing Sample (sample data missing)",
        0: "Not Significant (no significant difference)",
        100: "Missing Basis (basis data missing)",
        200: "Missing Basis and Sample (both basis and sample data missing)",
        300: "Significant Improvement (improved sample rate)"
    }
    
    return status_map.get(status_code, f"Unknown Status ({status_code})")


def main():
    parser = argparse.ArgumentParser(description='Fetch regression details by ID')
    parser.add_argument('regression_id', type=int, help='The regression ID to fetch')

    args = parser.parse_args()

    regression_id = args.regression_id
    url = f"https://sippy.dptools.openshift.org/api/component_readiness/regressions/{regression_id}"

    print(f"Fetching details for regression ID: {regression_id}")

    try:
        # Fetch regression data
        with urllib.request.urlopen(url) as response:
            regression_data = json.loads(response.read())

        # Initialize results dictionary
        results = {
            'regression_id': regression_id,
            'regression_data': regression_data,
            'test_details_data': None,
            'failed_sample_job_runs': []
        }

        # Check if test_details link exists
        if 'links' in regression_data and 'test_details' in regression_data['links']:
            test_details_url = regression_data['links']['test_details']

            with urllib.request.urlopen(test_details_url) as response:
                test_details_data = json.loads(response.read())

            results['test_details_data'] = test_details_data
        else:
            print("No test_details link found in regression data")

        # Print summary of fetched data
        print("\n=== Data Summary ===")
        print(f"Regression ID: {regression_id}")
        
        if 'test_name' in regression_data:
            print(f"Test Name: {regression_data['test_name']}")
        
        if 'release' in regression_data:
            print(f"Release: {regression_data['release']}")
        
        if 'base_release' in regression_data:
            print(f"Base Release: {regression_data['base_release']}")
        
        if 'component' in regression_data:
            print(f"Component: {regression_data['component']}")
        
        if 'capability' in regression_data:
            print(f"Capability: {regression_data['capability']}")
        
        if 'variants' in regression_data and regression_data['variants']:
            variants_str = ', '.join(regression_data['variants'])
            print(f"Variants: {variants_str}")
        
        if 'opened' in regression_data:
            print(f"Opened: {regression_data['opened']}")
        
        # Always print closed status - show time if valid, otherwise show "none"
        if ('closed' in regression_data and 
            isinstance(regression_data['closed'], dict) and 
            regression_data['closed'].get('valid') and 
            'time' in regression_data['closed']):
            print(f"Closed: {regression_data['closed']['time']}")
        else:
            print(f"Closed: none")
        
        if 'max_failure' in regression_data:
            print(f"Max Failure: {regression_data['max_failure']}")
        
        if results['test_details_data']:
            print(f"Test details fetched: Yes")
            
            # Display analysis data from the first item in analyses
            if 'analyses' in results['test_details_data'] and results['test_details_data']['analyses']:
                first_analysis = results['test_details_data']['analyses'][0]
                print(f"\n=== Analysis Data (First Item) ===")
                
                # Display status if available
                if 'status' in first_analysis:
                    status_code = first_analysis['status']
                    status_desc = get_status_description(status_code)
                    print(f"Status: {status_code} - {status_desc}")
                
                # Display sample_stats
                if 'sample_stats' in first_analysis:
                    sample_stats = first_analysis['sample_stats']
                    date_range = format_date_range(sample_stats)
                    print(f"Sample Stats{date_range}:")
                    print(f"  Success: {sample_stats.get('success_count', 'N/A')}")
                    print(f"  Failure: {sample_stats.get('failure_count', 'N/A')}")
                    print(f"  Flake: {sample_stats.get('flake_count', 'N/A')}")
                
                # Display base_stats
                if 'base_stats' in first_analysis:
                    base_stats = first_analysis['base_stats']
                    date_range = format_date_range(base_stats)
                    print(f"Base Stats{date_range}:")
                    print(f"  Success: {base_stats.get('success_count', 'N/A')}")
                    print(f"  Failure: {base_stats.get('failure_count', 'N/A')}")
                    print(f"  Flake: {base_stats.get('flake_count', 'N/A')}")
                
                # Collect failed sample job runs
                failed_sample_job_runs = []
                if 'job_stats' in first_analysis:
                    for job_stat in first_analysis['job_stats']:
                        if 'sample_job_run_stats' in job_stat and isinstance(job_stat['sample_job_run_stats'], list):
                            # sample_job_run_stats is an array, so iterate through it
                            for sample_job_run in job_stat['sample_job_run_stats']:
                                if ('test_stats' in sample_job_run and 
                                    'failure_count' in sample_job_run['test_stats']):
                                    
                                    failure_count = sample_job_run['test_stats']['failure_count']
                                    if failure_count > 0 and 'job_run_id' in sample_job_run:
                                        job_run_id = sample_job_run['job_run_id']
                                        failed_sample_job_runs.append(job_run_id)
                
                # Store failed sample job runs in results
                results['failed_sample_job_runs'] = failed_sample_job_runs
                
                # Display failed sample job runs
                if failed_sample_job_runs:
                    print(f"\nFailed Sample Job Runs ({len(failed_sample_job_runs)} total):")
                    for job_run_id in failed_sample_job_runs:
                        print(f"  {job_run_id}")
                else:
                    print(f"\nFailed Sample Job Runs: None")
            else:
                print("No analyses data found in test details")
        else:
            print(f"Test details fetched: No")

        return results

    except urllib.error.HTTPError as e:
        print(f"Error fetching regression: HTTP {e.code} - {e.reason}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
