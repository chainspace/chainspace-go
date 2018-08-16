#!/bin/bash

# generate the chainspace configuration
chainspace init testnet --shard-count 1 -c ./conf

# create ssh keys for gcloud
gcloud compute config-ssh
