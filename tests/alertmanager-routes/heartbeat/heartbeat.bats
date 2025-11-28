#!/usr/bin/env bats

load ../helper.bash

# Heartbeat

@test "heartbeats are delivered to Cronitor" {
  run amtool alertname="Heartbeat"
  assert_line --partial heartbeat
}
