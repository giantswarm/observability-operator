#!/usr/bin/env bats

load ../../alertmanager-routes-helper.bash

# Ticket alerts

@test "atlas ticket alerts are delivered to Github" {
  run amtool team=atlas severity=ticket
  assert_line --partial team_atlas_github
}

@test "phoenix ticket alerts are delivered to Github" {
  run amtool team=phoenix severity=ticket
  assert_line --partial team_phoenix_github
}

@test "shield ticket alerts are delivered to Github" {
  run amtool team=shield severity=ticket
  assert_line --partial team_shield_github
}

@test "rocket ticket alerts are delivered to Github" {
  run amtool team=rocket severity=ticket
  assert_line --partial team_rocket_github
}

@test "honeybadger ticket alerts are delivered to Github" {
  run amtool team=honeybadger severity=ticket
  assert_line --partial team_honeybadger_github
}

@test "tenet ticket alerts are delivered to Github" {
  run amtool team=tenet severity=ticket
  assert_line --partial team_tenet_github
}
