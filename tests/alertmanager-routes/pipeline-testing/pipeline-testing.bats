#!/usr/bin/env bats

load ../helper.bash

# Paging alerts on testing pipeline

@test "paging alerts with pipeline=testing are NOT delivered to PagerDuty" {
  run amtool team=foo severity=page pipeline=testing
  refute_line pagerduty-foo
}

@test "alerts with all_pipelines=true are delivered to PagerDuty" {
  run amtool team=foo all_pipelines=true
  assert_line --partial pagerduty-foo
}
