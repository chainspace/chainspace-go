kvstore
=======

How to test the key value storage.

Build the docker image for your contracts, you can do this using the makefile at the root of the repository:
```
$ make contract
```
In the future we could expect chainspace to pull the docker image directly from some docker registry

Then run the script `script/run-sharding/testnet`.
```
$ ./script/run-sharding/testnet N_SHARDS
```

This script will:
* initialize a new chainspace network with N shards
* start the different contracts required by your network (by default only the mock one)
* start the nodes of your network

The generated configuration expose the http rest api on node-1 on the port 8080.

In order to test the key value store you can use the httptest binary (which is installed at the same time as the chainspace binary when you run `make install`).

```
$ httptest -addr "0.0.0.0:8080" -workers 1 -objects 3 -duration 20
```
This will run the httptest for 20 second, with one worker creating 3 objects per transaction.

When it start running you should see a similar output on the standard output:
```
seeding objects for worker 0 with 0.0.0.0:8080
new seed key: djEAAAAA7rrTmXyvwexDRbDXWAU4n/gJPBJOkB8BiXYe4+VKmaNHpYCMrXNVoA2Siiau+e9ouPOZOG5CNLhiCDQ2KAzU+9+36tPibLbBwYx/B7M9TpGbDgD7VBL5XakoVf87VQWx
new seed key: djEAAAAAhXIV02TbSOW4+H1I/qOQ+8hOYnXRb/xVEAgkSzuEPgHM/BjK7g1Rv8IE5LmwmfxGnMrBMlO2XNX3W1wiZNxYkDB1ywDd210TGUt7Q7ZEqCqa/SCB7L3q6tfk2hy22cCU
new seed key: djEAAAAATS95HL0ehRGfXleJaTkfdR8SqFSwtC1G34YhoGRhu4gqeqi6LMlzVkxTkbN/niEXcQI7dpFwSfcVuUQBmfHWZf8ZRuNNhyDqWHiR2nOEb5Y1vNiQPu3PVepaoaJFYZN4
creating new object with label: label:0:0
creating new object with label: label:0:1
creating new object with label: label:0:2
seeds generated successfully
```

This show you the different labels which are created for the tests and associated to the objects, you can then use these to get the id / value of your objects.

Call this http endpoint in order to retrieve the chainspace object id associated to your label.
```
curl -v http://0.0.0.0:8080/kv/get-objectid -X POST -H 'Content-Type: application/json' -d '{"label": "label:0:1"}'
```


Call this http endpoint in ordet to retrieve the object associated to your label.
```
curl -v http://0.0.0.0:8080/kv/get -X POST -H 'Content-Type: application/json' -d '{"label": "label:0:1"}'
```

You can see that the object / object Id associated to your label evolve other time, when transactions are consuming them.
