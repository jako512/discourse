#!/bin/bash
# Run this script to set up the environment so you can rebuild the hooks
# executable. After running this script, you can run
#   go build hooks.go 
# to rebuild the hooks executable.

apt-get install -y golang-go git
export GOPATH=$HOME
go get gopkg.in/yaml.v1