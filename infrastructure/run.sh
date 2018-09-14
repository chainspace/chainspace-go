#!/bin/bash

cd /etc/chainspace/

nodeid=`cat /etc/chainspace/node_id`
echo "running node $nodeid"

# run genload on all nodes
./chainspace genload --initial-rate $1 --rate-decr $2 --rate-incr $3 --fixed-tps $4 --config-root /etc/chainspace/conf --mem-profile /etc/chainspace/conf/mem.pprof --cpu-profile /etc/chainspace/conf/cpu.pprof testnet `cat /etc/chainspace/node_id` > ~/log 2>&1 &

#uncomment to run genload on one node and normal nodes on the others

#if [ "$nodeid" -eq "1" ];then
#    ./chainspace genload --initial-rate $1 --rate-decr $2 --rate-incr $3 --fixed-tps $4 --config-root /etc/chainspace/conf --mem-profile /etc/chainspace/conf/mem.pprof --cpu-profile /etc/chainspace/conf/cpu.pprof testnet `cat /etc/chainspace/node_id` > ~/log 2>&1 &
#else
#    ./chainspace run --console-log info --file-log error --config-root /etc/chainspace/conf testnet `cat /etc/chainspace/node_id` > ~/log 2>&1 &
#fi

sleep 1
tail -f ~/log
