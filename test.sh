#!/usr/bin/env bash

GOEXPERIMENT=synctest go test -timeout 30s ./... -v
