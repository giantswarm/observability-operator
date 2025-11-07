#!/usr/bin/env bats

load ../../alertmanager-routes-helper.bash

# Paging alerts on stable pipeline

@test "paging alerts are delivered to OpsGenie" {
  run amtool team=foo severity=page
  assert_output opsgenie_router
}

@test "paging alerts with pipeline=stable delivered to PagerDuty" {
  run amtool team=foo severity=page pipeline=stable
  assert_output --partial pagerduty-foo
}

@test "paging alerts with all_pipelines are delivered to PagerDuty" {
  run amtool team=foo severity=page all_pipelines=true
  assert_output --partial pagerduty-foo
}

# Specific Atlas slack conditions

@test "atlas paging alerts are NOT delivered to Slack" {
  run amtool team=atlas severity=page
  refute_output team_atlas_slack
}

@test "atlas notify alerts are delivered to Slack" {
  run amtool team=atlas severity=notify
  assert_output --partial team_atlas_slack
}
