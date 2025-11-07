#!/usr/bin/env bats

load ../../alertmanager-routes-helper.bash

# Heartbeat

@test "heartbeats are delivered to OpsGenie" {
  run amtool alertname="Heartbeat"
  assert_output --partial heartbeat
}

@test "heartbeats are delivered to Cronitor" {
  run amtool alertname="Heartbeat"
  assert_output --partial cronitor-heartbeat
}
