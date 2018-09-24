#!/bin/bash

nodeid=`cat /etc/chainspace/node_id`
echo "running node $nodeid"
session=chainspace-${nodeid}


# cleaning old session
tmux kill-session -t ${session}
pkill chainspace
fuser -k 8080/tcp
rm -rf ~/.chainspace
sleep 2

# run sharding on all nodes
tmux new -d -s ${session}
tmux send-keys -t ${session} "/etc/chainspace/chainspace run --config-root /etc/chainspace/conf --mem-profile /etc/chainspace/conf/mem.pprof --cpu-profile /etc/chainspace/conf/cpu.pprof --console-log error testnet `cat /etc/chainspace/node_id` > ~/log 2>&1 &" "C-l" "C-m"
sleep 1
tail -f ~/log
