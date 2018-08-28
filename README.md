## Chainspace

This is a pre-alpha system comprising two main components:

* the `transactor` is a sharding component. It provides an implementation of the Sharded Byzantine Atomic Commit (S-BAC) protocol detailed in the Chainspace academic paper.
* the consensus component, which implements the leaderless consensus protocol detailed in the Blockmania paper.

Eventually, it's likely that we will split these two components. A project wanting only fast consensus, but no sharding, should be able use Blockmania by itself. For projects that need the added horizontal scalability of sharding, and are willing to pay the complexity cost, the S-BAC component can be added.

### Development Setup

Prerequisites:

* the `dep` package manager, see: https://golang.github.io/dep/docs/introduction.html. Dependencies are currently vendored, so there should be no need to run `dep ensure` before building.

Building:

Run `make install`. This will install the `chainspace` binary as well as the `httptest` load generator. You can generate a new set of shards, start the nodes, and hit them with a load test. See the help documentation (`chainspace -h` and `httptest -h`).

### Setting up and running nodes

The `chainspace init <networkname>` command, by default, creates a network consisting of 12 nodes grouped into 3 shards of 4 nodes each.

The setup you get from that is at present heavily skewed towards convenient development rather than production use. It will change as we get closer to production.

Have a look at the config files for the network you've generated (stored by default in `~/.chainspace/<networkname>`). The `network.yaml` file contains public signing keys and transport encryption certificates for each node in the network. Everything in `network.yaml` is public, and for the moment it defines network topology.

Each node also gets its own named configuration directory, containing:

* public and private signing and transport encryption keys for each node
* a node configuration file
* log output

As we have not yet implemented a directory server, seed nodes, or cryptographically secure peer discovery, nodes currently find each other on the local machine or local network using mDNS discovery. This is purely to aid in the development process (it means we don't need to manually configure a dozen ports for running several shards locally).

In the default setup, nodes 1, 4, 7, and 10 comprise shard 1. Run those node numbers if you're only interested in seeing consensus working. Otherwise, start all nodes to see sharding working as well.

### Sending transactions to consensus

TODO

### Sending transactions to shards

TODO

### Known Issues

* we're not cleaning up after ourselves by discarding objects used during consensus rounds. RAM usage correspondingly grows infinitely, and semi-prolonged usage will wipe out your machine. This is top of our list to fix once consensus is stable.
