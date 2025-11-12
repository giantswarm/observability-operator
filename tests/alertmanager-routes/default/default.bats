#!/usr/bin/env bats

load ../helper.bash

# Smoke test assessing validity of the test framework and default routing behavior

@test "default receiver is root" {
  run amtool foo=bar
  assert_line root
}
