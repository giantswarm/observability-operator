#!/usr/bin/env bats

load ../../alertmanager-routes-helper.bash

# Paging alerts on stable-testing pipeline

@test "paging alerts are delivered to OpsGenie" {
  run amtool team=foo severity=page
  assert_line opsgenie_router
}

@test "paging alerts with pipeline=stable-testing are delivered to PagerDuty" {
  run amtool team=foo severity=page pipeline=stable-testing
  assert_line --partial pagerduty-foo
}

@test "alerts with all_pipelines=true are delivered to PagerDuty" {
  run amtool team=foo all_pipelines=true
  assert_line --partial pagerduty-foo
}

# Ignored alerts

@test "workload cluster alerts are dropped" {
  run amtool cluster_type=workload_cluster
  assert_line blackhole
}

@test "test cluster alerts are dropped" {
  run amtool cluster_id=t-anything
  assert_line blackhole
}

@test "ClusterUnhealthyPhase alerts are dropped" {
  run amtool alertname=ClusterUnhealthyPhase
  assert_line blackhole
}

@test "WorkloadClusterApp alerts are dropped" {
  run amtool alertname=WorkloadClusterApp
  assert_line blackhole
}

# TODO: this route needs to be fixed as it would match anything which contains a any prefix contained in giantswarm
# e.g. namespace=org-anything would not be dropped as it contains "g"
# fix: namespace!=org-giantswarm,namespace=~"org-.+"
@test "ManagementClusterAppFailed alerts are dropped" {
  run amtool alertname=ManagementClusterAppFailed namespace=org-foo
  assert_line blackhole
}
