#!/usr/bin/env bats

load ../../alertmanager-routes-helper.bash

# Slack for Team Atlas

@test "atlas paging alerts are delivered to Slack" {
  run amtool team=atlas severity=page
  assert_line --partial team_atlas_slack
}

@test "atlas notify alerts are delivered to Slack" {
  run amtool team=atlas severity=notify
  assert_line --partial team_atlas_slack
}

@test "atlas inhibition are not delivered to slack" {
  run amtool team=atlas severity=notify alertname=Inhibition
  assert_line root
}

@test "atlas heartbeats are not delivered to slack" {
  run amtool team=atlas severity=notify alertname=Heartbeat
  assert_line root
}

# Slack for Team Phoenix

@test "phoenix silent sloth alerts are delivered to slack" {
  run amtool team=phoenix sloth_severity=page silence=true
  assert_line team_phoenix_slack
}

# Slack for Team Shield

@test "shield page alerts are delivered to slack" {
  run amtool team=shield severity=page
  assert_line team_shield_slack
}

@test "shield notify alerts are delivered to slack" {
  run amtool team=shield severity=notify
  assert_line team_shield_slack
}

# Slack for Team Rocket

@test "rocket page alerts are delivered to slack" {
  run amtool team=rocket severity=page
  assert_line team_rocket_slack
}

@test "rocket notify alerts are delivered to slack" {
  run amtool team=rocket severity=notify
  assert_line team_rocket_slack
}

# Slack for Team Honeybadger

@test "honeybadger page alerts are delivered to slack" {
  run amtool team=honeybadger severity=page
  assert_line team_honeybadger_slack
}

@test "honeybadger notify alerts are delivered to slack" {
  run amtool team=honeybadger severity=notify
  assert_line team_honeybadger_slack
}

# Slack for Team Tenet

@test "tenet notify alerts are delivered to slack" {
  run amtool team=tenet severity=notify
  assert_line team_tenet_slack
}

# Slack for Flack alerts

@test "flaco alerts are delivered to slack" {
  run amtool alertname=Falco
  assert_line falco_noise_slack
}

@test "flaco alerts with suffix are delivered to slack" {
  run amtool alertname=FalcoAnything
  assert_line falco_noise_slack
}
