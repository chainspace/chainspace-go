How to
======

First cross compile the chainspace binary for the gcp target, a rule exists in the Makefile at the root:
```
$ cd .. && make gcp && cd -
```
This should have generated a chainspace.upx in the infrastructure folder.

Then generate the configuration for the network you want to deploy, e.g for 7 nodes (do this in the infrastructure directory):
```
$ chainspace init testnet --registry acoustic-atom-211511.appspot.com --shard-count 1 --shard-size 7 --disable-transactor true --config-root ./conf --http-port 8080
```

Then run terraform. You can customize variable in terraform using environnment variable. Some are setup in the .envrc file (to use with direnv). You should edit the variable "TF_VAR_node_count" to match the number of node for the network you just created, currently 7.

You can now run terraform, you may need to setup terraform to work with gcloud.
```
# in order to deploy the network in the europe-west2-b zone (London)
$ terraform apply -target "google_compute_instance_from_template.genload"

# to deploy in multiple zones
$ terraform apply -target "google_compute_instance_from_template.genloadmulti"
```

Once the infrastructure is deployed with success you can now run the script which will run the nodes in the gcp instances, using as parameter the number of node in your network:
```
# if you deployed in London
$ ./scripts/rungenload 7

# or in multiple zones
$ ./scripts/rungenloadmulti 7
```

Then to destroy the infrastructure just run:
```
$ terraform destroy
```
