#!/bin/bash

cd /etc/chainspace/

nodeid=`cat /etc/chainspace/node_id`
echo "running node $nodeid"

# run genload on all nodes
./chainspace genload --initial-rate $1 --rate-decr $2 --rate-incr $3 --expected-latency $4 --config-root /etc/chainspace/conf --mem-profile /etc/chainspace/conf/mem.pprof --cpu-profile /etc/chainspace/conf/cpu.pprof testnet `cat /etc/chainspace/node_id` > ~/log 2>&1 &


sleep 1
tail -f ~/log
