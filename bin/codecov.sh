#!/usr/bin/env bash
set -e
set -u
set -x
SCRIPTPATH=$( cd "$(dirname "$0")" ; pwd -P )
ROOTDIR=$(dirname $SCRIPTPATH)
cd $ROOTDIR

echo "Code coverage test"
TMPDIR=$(mktemp -d)

# coverage test needs to run one package per command.
# This script runs nproc/2 in parallel.
# Script fails if any one of the tests fail.
# FIXME: Bootstrapgen test can only be run with bazel at this time,
# It is excluded from the test packages.

i=0
# half the number of cpus seem to saturate
if [[ -z ${maxprocs:-} ]];then
  maxprocs=$[$(getconf _NPROCESSORS_ONLN)/2]
fi
num=0
pids=""
declare -A pkgs

for d in $(go list ./... | grep -v vendor); do
    i=$[$i+1]
    #FIXME remove mixer tools exclusion after tests can be run without bazel
    if [[ $d == "istio.io/istio/tests"* || \
      $d == "istio.io/istio/mixer/tools/codegen"* ]];then
      echo "Skipped $d"
      continue
    fi
    go test -coverprofile=$TMPDIR/$i $d &
    pid=$!
    pkgs[$pid]=$d
    pids+=" $pid"
    num=$(jobs -p|wc -l)
    while [ $num -gt $maxprocs ]
    do
      sleep 2
      num=$(jobs -p|wc -l)
    done
done
touch $TMPDIR/empty
cat $TMPDIR/* > coverage.txt

ret=0
for p in $pids; do
    if ! wait $p; then
        echo "${pkgs[$p]} failed"
        ret=1
    fi
done

exit $ret
