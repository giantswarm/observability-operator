#!/usr/bin/env bats

load ../../alertmanager-routes-helper.bash

# Paging alerts on testing pipeline

@test "paging alerts are delivered to OpsGenie" {
  run amtool team=foo severity=page
  assert_output opsgenie_router
}

@test "paging alerts with pipeline=testing are NOT delivered to PagerDuty" {
  run amtool team=foo severity=page pipeline=testing
  refute_output pagerduty-foo
}

@test "alerts with all_pipelines=true are delivered to PagerDuty" {
  run amtool team=foo all_pipelines=true
  assert_output --partial pagerduty-foo
}
