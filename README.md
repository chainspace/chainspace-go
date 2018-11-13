## Chainspace

[![pipeline status](https://code.constructiveproof.com/chainspace/prototype/badges/master/pipeline.svg)](https://code.constructiveproof.com/chainspace/prototype/commits/master) [![coverage report](https://code.constructiveproof.com/chainspace/prototype/badges/master/coverage.svg)](https://code.constructiveproof.com/chainspace/prototype/commits/master)

Chainspace is a smart contract system offering speedy consensus and unlimited horizontal scalability.

At present, our running code has two main components:

* the `sbac` is a sharding component. It provides an implementation of the Sharded Byzantine Atomic Commit (S-BAC) protocol detailed in the [Chainspace](https://arxiv.org/abs/1708.03778) academic paper.
* the consensus component, which implements the leaderless consensus protocol detailed in the [Blockmania](https://arxiv.org/abs/1809.01620) paper.

Eventually, it's likely that we will split these two components. A project wanting only fast consensus, but no sharding, should be able use Blockmania by itself. For projects that need the added horizontal scalability of sharding, the S-BAC component would be added. But for the moment, the two components co-exist in the same codebase.

## Development Setup

There are several prerequisites to get a running system working:

* [Go](https://golang.org/dl/) `1.11` or above. Earlier versions won't work
* [Docker](https://docs.docker.com/install/)

To test that Docker is working run `docker run hello-world` in a shell. Fix any errors before proceeding.

With these requirements met, run `make install`. This will build and install the `chainspace` binary as well as the `httptest` load generator. You can generate a new set of shards, start the nodes, and hit them with a load test. See the help documentation (`chainspace -h` and `httptest -h`) for each binary.

### Git Flow and Versioning

If you are of the Mac persuasion you should install [Git Flow AVH](https://github.com/petervanderdoes/gitflow-avh) rather than the standard Git Flow. To do this using [Brew](https://brew.sh/) you'd go:

```sh
brew update && brew upgrade
brew install git-flow-avh
```

The version of the application should be set in the `VERSION` file found in the root of the project. This will be updated the automatically when creating either a `release` or `hotfix` providing you have Git Flow and run `git flow init` on your project.

Use the defaults for everything except the `Hooks and filters directory?` which should be set to  `{PATH_TO_REPO_DIR}/hooks`. The `hooks` directory is a Git submodule of these handy [Git Flow Hooks](https://github.com/jaspernbrouwer/git-flow-hooks) which include the auto updating of the `VERSION` file.

```sh
git flow init

Which branch should be used for bringing forth production releases?
   - master
Branch name for production releases: [master]
Branch name for "next release" development: [develop]

How to name your supporting branch prefixes?
Feature branches? [feature/]
Bugfix branches? [bugfix/]
Release branches? [release/]
Hotfix branches? [hotfix/]
Support branches? [support/]
Version tag prefix? []
Hooks and filters directory? [{PATH_TO_REPO_DIR}/.git/hooks] # Set this to {PATH_TO_REPO_DIR}/hooks
```

NOTE: `{PATH_TO_REPO_DIR}` should be the current location of the repo on your local machine.

For more information on Git Flow, here is a handy [Git Flow Cheatsheat](https://danielkummer.github.io/git-flow-cheatsheet/).

## Setting Up and Running Nodes

The `chainspace init <networkname>` command, by default, creates a network consisting of 12 nodes grouped into 3 shards of 4 nodes each.

The setup you get from that is heavily skewed towards convenient development rather than production use. It will change as we get closer to production.

Have a look at the config files for the network you've generated (stored by default in `~/.chainspace/<networkname>`). The `network.yaml` file contains public signing keys and transport encryption certificates for each node in the network. Everything in `network.yaml` is public, and for the moment it defines network topology. Later, it will be replaced by a directory component.

Each node also gets its own named configuration directory, containing:

* public and private signing and transport encryption keys for each node
* a node configuration file
* log output

In the default setup, nodes 1, 4, 7, and 10 comprise shard 1. Run those node numbers if you're only interested in seeing consensus working. Otherwise, start all nodes to see sharding working as well.

```bash
rm -rf ~/.chainspace # destroy any old configs, make you sad in production
chainspace init foonet
chainspace run foonet 1
chainspace run foonet 4
chainspace run foonet 7
chainspace run foonet 10
```

A convenient script runner is included. The short way to run it is:

```bash
rm -rf ~/.chainspace # clear previous configs, superbad idea in production
chainspace init foonet
script/run-testnet foonet
```

This will fire up a single shard which runs consensus, and make it available for use.

## Peer Discovery

We have not yet implemented seed nodes, or a cryptographically secure method of peer discovery. This is for future development and the details aren't yet clear (although we're working on it). So we have implemented some simple methods for stitching together networks until we are ready to commit to a final system.

Nodes currently find each other in two ways:

1. mDNS discovery
1. registry

### mDNS discovery

In development or on private networks, nodes can discover each other using mDNS broadcast. This allows zero-configuration setups for nodes that are all on the same subnet.

Use the Registry when configuring nodes across the public internet. We run a public registry at https://registry.chainspace.io

You can run your own Registry if you want. Init your network with the `--registry` flag if you plan to use a Registry server:

```bash
chainspace init foonet --registry registry.chainspace.io
```

The Registry will then appear in each node's `node.yaml`:

```yaml
registries:
- host: registry.chainspace.io
- token: 05b16f5d45377baff52c25e2c154a00b126f7b75b7345794d3e15535b49a03f955b9c355
```

The randomly-generated registry `token` ensures that unique shared secret is used on a per-network basis so that multiple networks can share the same registry without any additional setup.

Nodes will automatically register themselves with the network's registry server when they start up.

It is possible to use both the Registry and mDNS discovery at the same time.

## Sending Transactions To Consensus

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

## Consensus Load Generator

The `chainspace genload <networkname> <nodenumber>` command starts up the specified node, and additionally a client which floods the consensus interface (in Go) with simulated transactions (100 bytes by default).

To get some consensus performance numbers, run this in 4 separate terminals:

```bash
rm -rf ~/.chainspace
chainspace init foonet
chainspace genload foonet 1
chainspace genload foonet 4
chainspace genload foonet 7
chainspace genload foonet 10
```

The client keeps increasing load until it detects that the node is unable to handle the transaction rate, based on timing drift when waking up to generate epochs. At that point the client backs off, and in general a stable equilibrium is reached. The `genload` console logs then report on average, current, and highest transactions per second throughput.

A convenient script runner is included. The short way to run it is:

```bash
rm -rf ~/.chainspace
chainspace init foonet
script/genload-testnet foonet
```

This will start nodes 1, 4, 7 and 10 in `tmux` terminals, pumping transactions through automatically.

*NOTE: to get valid results, turn off all other applications when doing performance testing. Also disable swap on your system, turn off CPI BIOS thermal controls, disable power management, and don't run it on battery power.*

Running `chainspace genload` with swap enabled can cause system lockups on Linux, as the system thinks much more RAM is available than is in fact the case, write latencies increase drastically once swapping starts, and the system freaks out. `sudo swapoff -a` is your friend, with `sudo swapon -a` to get your swap back when you're done running Chainspace.

## Sending Transactions To Shards

TODO: Write this...

## REST Documentation

Many parts of the system are available to poke at via a RESTful HTTP server interface. After starting a node locally, you can see what's available by going to http://localhost:8080/docs

TODO: we still need to really document how to use the REST endpoints from a conceptual standpoint.

## Adding Dependencies

Dependency management is done via `go mod`. See the latest Go docs for a primer on that if you're doing development. `go mod help` should give you the basics.
