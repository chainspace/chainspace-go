#/bin/bash

expected_txs_per_secs=3500
expected_latency=3.7
network_name=testnet
shard_size=4
shard_count=1

session="blockmaniatest-${network_name}"

# remove previous runs configurations
rm -rf ~/.chainspace/${session}

echo ">> initializing chainspace network"
# initialize chainspace network
chainspace init ${session} --shard-count ${shard_count} --shard-size ${shard_size} --disable-sbac

# start all the nodes
tot_node="$((${shard_count} * ${shard_size}))";
for i in $(seq 1 ${tot_node}); do
    echo ">> running blockmaniatest node ${i}"
    blockmaniatest -network ${session} -nodeid ${i} &
    pids[${i}]=$!
    echo "blockmania node ${i} started with PID=${pids[${i}]}"
done

# wait for the tests to finish
for pid in ${pids[*]}; do
    # wait for one of the process to finish, whatever the finish order
    echo ">> waiting for PID=${pid} to finish"
    wait $pid

    # exit if the process exited with errors
    if [ "$?" != "0" ]; then
	echo "PID=${pid} exited with error code $?"
	exit 1
    fi
done

# analysis results

tot_node="$((${shard_count} * ${shard_size}))";
for i in $(seq 1 ${tot_node}); do
    echo ">> checking testresults for blockmaniatest node ${i}"

    # check txs per secs
    txspersecs=`cat ~/.chainspace/testresults/blockmaniatest-results-node${i}.json | jq -r .avg_txs`
    if [ ${txspersecs} -lt ${expected_txs_per_secs} ]; then
	echo "blockmania node ${i} produced ${txspersecs} txs expected ${expected_txs_per_secs}"
	exit 1
    fi

    #check txs avg latency
    txslatency=`cat ~/.chainspace/testresults/blockmaniatest-results-node${i}.json | jq -r .avg_latency`
    if (( $(echo "${txslatency} > ${expected_latency}" |bc -l) )); then
	echo "blockmania node ${i} had a latency of ${txslatency}s expected ${expected_latency}s"
	exit 1
    fi

    echo "blockmania node ${i} produced ${txspersecs}tx/s and had a latency of ${txslatency}s pass=OK"
done
