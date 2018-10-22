kvstore
=======

TODO: Add a description of what the Key Value store is and does, how it relates to SBAC, contracts, etc.

How to test the key value storage.

Prerequisites
-------------

* Install [Docker](https://docs.docker.com/install/) for your platform
* Test that Docker is working. Run `docker run hello-world` in a shell. Fix any errors before proceeding.
* Set up a testnet

Building the Key Value store
---------------

Build the docker image for your contracts, you can do this using the makefile at the root of the repository:

```
$ make contract
```
In the future chainspace will pull the docker image directly from a docker registry according to which contract is being run. At present during development we've simply hard-coded in a dummy contract inside a Docker container we've made.  

Next, run the script `script/run-sharding-testnet`.
```
$ ./script/run-sharding-testnet 1
```

This script will:

* initialize a new chainspace network `testnet-sharding-1` with `1` shard.
* start the different contracts required by your network (by default only the dummy contract)
* start the nodes of your network

The generated configuration exposes a HTTP REST API on all nodes. Node 1 starts on port 8001, Node 2 on port 8002 and so on.

Seeding trial objects using httptest
-------------------------------------

In order to test the key value store you can use the `httptest` binary (which is installed at the same time as the chainspace binary when you run `make install`).

```
$ httptest -addr "0.0.0.0:8001" -workers 1 -objects 3 -duration 20
```

This will run `httptest` for 20 seconds, with one worker creating 3 objects per transaction.

When it starts running you should see output like this on stdout:
```
seeding objects for worker 0 with 0.0.0.0:8001
new seed key: djEAAAAA7rrTmXyvwexDRbDXWAU4n/gJPBJOkB8BiXYe4+VKmaNHpYCMrXNVoA2Siiau+e9ouPOZOG5CNLhiCDQ2KAzU+9+36tPibLbBwYx/B7M9TpGbDgD7VBL5XakoVf87VQWx
new seed key: djEAAAAAhXIV02TbSOW4+H1I/qOQ+8hOYnXRb/xVEAgkSzuEPgHM/BjK7g1Rv8IE5LmwmfxGnMrBMlO2XNX3W1wiZNxYkDB1ywDd210TGUt7Q7ZEqCqa/SCB7L3q6tfk2hy22cCU
new seed key: djEAAAAATS95HL0ehRGfXleJaTkfdR8SqFSwtC1G34YhoGRhu4gqeqi6LMlzVkxTkbN/niEXcQI7dpFwSfcVuUQBmfHWZf8ZRuNNhyDqWHiR2nOEb5Y1vNiQPu3PVepaoaJFYZN4
creating new object with label: label:0:0
creating new object with label: label:0:1
creating new object with label: label:0:2
seeds generated successfully
```

This shows you the different labels which are created by the tests and associated to objects. You can then use these to get the id / value of your objects.

Retrieving objects
------------------

Call this http endpoint in order to retrieve the chainspace object id associated to your label.
```
curl -v http://0.0.0.0:8001/kv/get-objectid -X POST -H 'Content-Type: application/json' -d '{"label": "label:0:1"}'
```


Call this http endpoint in ordet to retrieve the object associated to your label.
```
curl -v http://0.0.0.0:8001/kv/get -X POST -H 'Content-Type: application/json' -d '{"label": "label:0:1"}'
```

You can see that the object / object Id associated to your label evolves over time, when transactions are consuming them.


Running multiple shards
-----------------------

This should work fine, but there's a caveat. When
