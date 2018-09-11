## Chainspace

This is a pre-alpha smart contract system comprising two main components:

* the `transactor` is a sharding component. It provides an implementation of the Sharded Byzantine Atomic Commit (S-BAC) protocol detailed in the Chainspace academic paper.
* the consensus component, which implements the leaderless consensus protocol detailed in the Blockmania paper.

Eventually, it's likely that we will split these two components. A project wanting only fast consensus, but no sharding, should be able use Blockmania by itself. For projects that need the added horizontal scalability of sharding, the S-BAC component would be added. But for the moment, the two components co-exist in the same codebase.

### Development Setup

You'll need Go `1.11`. This is relatively new, don't try to use anything older.

Run `make install`. This will build and install the `chainspace` binary as well as the `httptest` load generator. You can generate a new set of shards, start the nodes, and hit them with a load test. See the help documentation (`chainspace -h` and `httptest -h`) for each binary.

### Setting up and running nodes

The `chainspace init <networkname>` command, by default, creates a network consisting of 12 nodes grouped into 3 shards of 4 nodes each.

The setup you get from that is at present heavily skewed towards convenient development rather than production use. It will change as we get closer to production.

Have a look at the config files for the network you've generated (stored by default in `~/.chainspace/<networkname>`). The `network.yaml` file contains public signing keys and transport encryption certificates for each node in the network. Everything in `network.yaml` is public, and for the moment it defines network topology.

Each node also gets its own named configuration directory, containing:

* public and private signing and transport encryption keys for each node
* a node configuration file
* log output

In the default setup, nodes 1, 4, 7, and 10 comprise shard 1. Run those node numbers if you're only interested in seeing consensus working. Otherwise, start all nodes to see sharding working as well.

### Peer discovery

We have not yet implemented seed nodes, or cryptographically secure method of peer discovery. This is for future development and the details aren't yet clear. So we have implemented some simple methods for stitching together networks until we are ready to commit to a final system.

Nodes currently find each other in two ways:

1. mDNS discovery
1. registry

#### mDNS discovery

In development or on private networks, nodes can discover each other using mDNS broadcast. This allows zero-configuration setups for nodes that are all on the same subnet.

Use the Registry when configuring nodes across the public internet. We run a public registry at https://registry.chainspace.io

You can run your own Registry if you want. Init your network with the `--registry` flag if you plan to use a Registry server:

`chainspace init kickass --registry registry.chainspace.io`

The Registry will then appear in each node's `node.yaml`:

```yaml
registries:
- host: registry.chainspace.io
  token: 05b16f5d45377baff52c25e2c154a00b126f7b75b7345794d3e15535b49a03f955b9c355
```

The randomly-generated registry `token` ensures that unique shared secret is used on a per-network basis so that multiple networks can share the same registry without any additional setup.

Nodes will automatically register themselves with the network's registry server when they start up.

It is possible to use both the Registry and mDNS discovery at the same time.

### Sending transactions to consensus

At the moment, there is no externally-exposed network interface for using the consensus component by itself. You can however access it programmatically using Go, e.g.

```go
import "chainspace.io/prototype/node"

s, err := node.Run(cfg)
if err != nil {
  log.Fatal(err)
}
s.Broadcast.AddTransaction(txdata, fee)
```

[DAVE TODO: check that code can actually run]

### Sending transactions to shards


### Consensus load generator

The `chainspace genload <networkname> <nodenumber>` command starts up the specified node, and additionally a client which floods the consensus interface (in Go) with transactions (100 bytes by default). The client keeps increasing load until it detects that the node is unable to handle the transaction rate, based on timing drift when waking up to generate epochs. At that point the client backs off, and in general a stable equilibrium is reached. The `genload` console logs then report on average, current, and highest transactions per second throughput.

*NOTE: to get valid results, turn off all other applications when doing performance testing. Also disable swap on your system, turn off CPI BIOS thermal controls, disable power management, and don't run it on battery power.*

Running `chainspace genload` with swap enabled can cause system lockups on Linux, as the system thinks much more RAM is available than is in fact the case, write latencies increase drastically once swapping starts, and the system freaks out. `sudo swapoff -a` is your friend, with `sudo swapon -a` to get your swap back when you're done running Chainspace.

### Adding dependencies

Dependency management is done via `go mod`. See the latest Go docs for a primer on that if you're doing development. `go mod help` should give you the basics.
