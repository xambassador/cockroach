#!/bin/sh

# Copyright 2020 The Cockroach Authors.
#
# Use of this software is governed by the CockroachDB Software License
# included in the /LICENSE file.


set -euo pipefail

psql 'postgresql://postgres:postgres@127.0.0.1?sslmode=disable' \
	-c 'create table if not exists metrics(name varchar(255), time timestamp, source varchar(255), value real)' \
	-c 'truncate table metrics;' \
	-c 'copy metrics(name, time, source, value) from stdin csv;' < /dev/stdin

