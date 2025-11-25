#!/usr/bin/env bats

load ../helper.bash

# Ticket alerts

@test "atlas ticket alerts are delivered to Github" {
  run amtool team=atlas severity=ticket
  assert_line --partial github_atlas
}

@test "phoenix ticket alerts are delivered to Github" {
  run amtool team=phoenix severity=ticket
  assert_line --partial github_phoenix
}

@test "shield ticket alerts are delivered to Github" {
  run amtool team=shield severity=ticket
  assert_line --partial github_shield
}

@test "rocket ticket alerts are delivered to Github" {
  run amtool team=rocket severity=ticket
  assert_line --partial github_rocket
}

@test "honeybadger ticket alerts are delivered to Github" {
  run amtool team=honeybadger severity=ticket
  assert_line --partial github_honeybadger
}

@test "tenet ticket alerts are delivered to Github" {
  run amtool team=tenet severity=ticket
  assert_line --partial github_tenet
}
