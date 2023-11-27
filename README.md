# Data Distribution Agent

[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/coatyio/dda.svg)](https://github.com/coatyio/dda/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/coatyio/dda)](https://goreportcard.com/report/github.com/coatyio/dda)
[![Coverage Report](https://coatyio.github.io/dda/coverage-badge.svg)](https://coatyio.github.io/dda/coverage.html)
[![Go Reference](https://pkg.go.dev/badge/github.com/coatyio/dda.svg)](https://pkg.go.dev/github.com/coatyio/dda)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/coatyio/dda/blob/main/LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/coatyio/dda)](https://github.com/coatyio/dda/releases/latest)

## Table of Contents

* [Introduction](#introduction)
* [Overview](#overview)
* [Data-Centric Communication](#data-centric-communication)
* [Quick Start](#quick-start)
  * [Prebuilt Binaries](#prebuilt-binaries)
  * [Docker Image](#docker-image)
* [Examples](#examples)
* [Contributing](#contributing)
* [License](#license)

## Introduction

This project realizes the Data Distribution Agent (DDA), a loosely coupled
multi-agent system that enables unified and universal data-centric communication
and collaboration in distributed and decentralized applications. Its main goal
is to remove the inherent complexity in developing communication technologies
for distributed systems by exposing _Data-centric Communication as a Service_ to
application developers without requiring in-depth knowledge of messaging
protocols and communication networks. DDA is designed for universal data
exchange, enabling human-to-human, machine-to-machine, or hybrid communication.

DDA follows the _Sidecar_/_Sidekick_ decomposition design pattern, where each
DDA sidecar is attached to a primary application component typically co-located
on the same host. The application component provides the domain-specific core
functionality of a single logical node within the distributed system. The DDA
sidecar provides unified peripheral services to its local application component
including generic data-centric patterns for communication between components,
distributed state synchronization, and local persistent storage. DDA sidecar
services are exposed by modern open-standard gRPC APIs that can be consumed
conveniently by application components written in different languages using
different frameworks, including low-code platforms. A DDA sidecar is independent
from its primary application component in terms of runtime environment and
programming language and can be easily used with containers.

DDA sidecars are very lightweight and work across edge, cloud, on-premise, and
hybrid environments, either as processes or containerized. Technically, a DDA
sidecar is running as a standalone platform-specific binary prebuilt from pure
Go. It is available on a variety of platforms, including Linux, macOS, and
Windows.

As an alternative to the sidecar pattern, DDA functionality is also available as
a lightweight library (Golang reference implementation) which can be directly
embedded into a primary application component written in Go. The DDA library
enables a deeper level of integration and optimized service invocation with less
overhead, notably latency in calls. This approach is particularly suitable for
latency sensitive small footprint components on constrained devices where the
resource cost of deploying a sidecar for each instance is not worth the
advantage of isolation.

We believe DDA is a major step forward in practice that allows developers to
create more powerful collaborative applications with less complexity and in less
time, running anywhere.

DDA comes with complete reference documentation, a developer guide, and a
collection of ready-to-run best practice code examples (for details see
[here](https://coatyio.github.io/dda)).

The software design and coding principles on which DDA is based follow the MAYA
(Most Advanced. Yet Acceptable.) design strategy and the YAGNI (You Aren’t Gonna
Need It) principle.

## Overview

A DDA exposes a collection of application-level peripheral services that
_abstract_ commonly used functionality in decentralized applications:

* data-centric communication over interchangeable messaging protocols
* distributed state synchronization based on consensus algorithms that
  guarantee strong consistency
* local persistent key-value storage

Application components that do not embed the DDA library directly can consume
these services through modern open-standard APIs based on:

* [gRPC](https://grpc.io/) - A high performance, open source RPC framework
  supporting a wide range of programming languages and platforms with a common
  declarative semantic API protocol
* [gRPC-Web](https://github.com/grpc/grpc-web) - A JavaScript implementation of
  gRPC for browser clients

In addition, a DDA supports observability, i.e. the ability to measure and infer
its internal state by analyzing [OpenTelemetry](https://opentelemetry.io)
traces, logs, and metrics that it generates and exports.

## Data-Centric Communication

Utilizing DDA, you can build a distributed application with decoupled
data-centric interaction between decentrally organized application components,
which are loosely coupled and communicate with each other in (soft) real-time
over interchangeable open-standard publish-subscribe messaging protocols, such
as MQTT 5 (reference implementation), Zenoh, Kafka, or DDS (on the roadmap).
Providing a generic abstraction layer on top of interchangeable messaging
protocols avoids potential vendor lock-in and allows you to choose a protocol
that best fits your existing networking infrastructure.

A typical usage is in IoT prosumer scenarios where smart components act in an
autonomous, collaborative, and ad-hoc fashion. You can dynamically and
spontaneously add new components and new features without distracting the
existing system in order to adapt to your ever-changing scenarios. All types of
components such as sensors, mobile apps, edge and cloud services can be
considered equivalent.

At its core, a DDA exposes a minimal, yet complete set of communication patterns
for application components to talk to each other by routed one-way/two-way and
one-to-many/many-to-one communication flows without the need to know about each
other. Subject of communication is domain-specific data transmitted to
interested parties in the form of:

* a one-way _Event_ to route _data in motion_ from an event producer (source) to
  interested event consumers,
* a two-way _Action_ to issue a remote operation with _data in use_ to be
  executed by interested actors and to receive its results,
* a two-way _Query_ to query remote and/or distributed _data at rest_ from
  interested retrieving components.

Structured Event, Action, and Query data is described in a common way adhering
closely to the [CloudEvents](https://cloudevents.io/) specification. Thereby, a
DDA combines the characteristics of both classic request-response and
publish-subscribe communication to make data in motion, data in use, and data at
rest available throughout the application by expressing interest in them. The
two-way patterns support multiple short- and long-lived responses over time
and/or over parties enabling sophisticated features such as querying live data
which is updated over time or receiving progressive results for a remote
operation call. In contrast to classic client-server systems, all DDAs are equal
in that they can act both as producers/requesters and consumers/responders.

One of the unique features of DDA communication is the fact that a single Action
or Query request in principle can yield multiple responses over time, even from
the same responder. The use case specific logic implemented by the requester
determines how to handle such responses. For example, the requester can decide
to

* just take the first response and ignore all others,
* only take responses received within a given time interval,
* only take responses until a specific condition is met,
* handle any response over time, or
* process responses as defined by any other application-specific logic.

We think decentralized collaborative applications often have a natural need for
such powerful forms of interaction. Restricting interaction to strongly coupled
one-to-one communication (classic RPC, REST) quickly becomes complex,
inflexible, and unmaintainable. Restricting interaction to loosely coupled
one-way communication (classic publish-subscribe) lacks the power of
transmitting information in direct responses. Which is why a DDA combines the
best of both worlds to allow developers to create more powerful collaborative
applications with less complexity and in less time.

Using the minimal set of communication patterns, higher-level patterns of
interaction and collaboration between decentralized application components can
be composed, e.g.

* coordinating dynamic workflows, workloads, and computations over a set of
  distributed workers,
* distributing and synchronizing shared state,
* discovering and tracking remote application entities,
* negotiating specialized side channels for streaming high-volume and
  high-frequency data,
* handling backpressure in producer-consumer scenarios by pull-based approaches,
* all types of auctioning, such as a first-price sealed-bid auction (aka blind
  auction) where each bidder submits a sealed bid and the best bidder wins.

While some of these patterns will be integrated into DDA as generic peripheral
services in the near future, more domain-specific ones are left to dedicated
applications.

## Quick Start

To run a DDA sidecar, you may use

* a prebuilt standalone platform-specific binary, or
* a Docker image.

In both cases, the dda program is configured by a YAML configuration file
deployed along with it. A fully documented default configuration file `dda.yaml`
is part of all DDA sidecar
[deployments](https://github.com/coatyio/dda/releases). For details on how to
provide a configuration file run the binary or image with the `-h` command line
option.

To use DDA as a library, refer to the [DDA Developer
Guide](https://coatyio.github.io/dda/DEVGUIDE.html).

### Prebuilt Binaries

Use one of the prebuilt platform-specific binaries delivered as [GitHub
release](https://github.com/coatyio/dda/releases) assets.

Note that each GitHub release asset also includes a default DDA configuration
file and definition files of the public DDA APIs, i.e. Protobuf files and
JavaScript stubs to consume the DDA sidecar services as a client using gRPC or
gRPC-Web, respectively.

Alternatively, in a Golang development environment you can build and run the DDA
module from source using the Golang toolchain:

```sh
go install github.com/coatyio/dda/cmd/dda@latest

dda -h
```

### Docker Image

Use one of the versioned Docker images deployed in the [GitHub Container
Registry](https://ghcr.io/coatyio/dda). The DDA configuration file `dda.yaml`
and associated assets, such as certificates, must be bind-mounted on path `/dda`
in the container. Configured DDA API service ports must be exposed by the
container:

```sh
docker run --rm -p 8800:8800 -p 8900:8900 -v /path-to-config-folder:/dda ghcr.io/coatyio/dda:<release-version>
```

## Examples

This project is accompanied by
[dda-examples](https://github.com/coatyio/dda-examples), a repository that
provides a collection of ready-to-run best practice code examples demonstrating
how to use DDA as a sidecar or library in a decentralized desktop or web
application:

* `compute` - a pure Go app with distributed components utilizing either the DDA
  library directly or a co-located DDA sidecar over gRPC
* `light-control` - a distributed web app in JavaScript/HTML connecting to its
  co-located DDA sidecar over gRPC-Web

For detailed documentation take a look at the README of the individual example
projects and delve into the [DDA Developer
Guide](https://coatyio.github.io/dda/DEVGUIDE.html).

## Contributing

Contributions to DDA are welcome and appreciated. Please follow the recommended
practice for idiomatic Go [programming](https://go.dev/doc/effective_go) and
[documentation](https://tip.golang.org/doc/comment).

The DDA project is organized as a single Go module which consists of multiple
packages that realize functionality of individual peripheral DDA services and
its open APIs. As a contributor, you may want to add new DDA services, add
features to existing services, apply patches, and publish new releases.

To set up the project on your developer machine, install a compatible
[Go](https://go.dev/) version as specified in the `go.mod` file.

Next, install the [Task](https://taskfile.dev/) build tool, either by using one
of the predefined binaries delivered as [GitHub
release](https://github.com/go-task/task/releases) assets or by using the Golang
toolchain:

```sh
go install github.com/go-task/task/v3/cmd/task@latest
```

Run `task install` to install Go tools required to build, test, and release DDA,
and to install DDA module dependencies.

All build, test, and release related tasks can be performed using the Task tool.
For a list of available tasks, run `task`.

> __Note__: If you intend to make changes to Protobuf service definitions,
> install the latest release of the [Protobuf
> compiler](https://grpc.io/docs/protoc-installation/#install-pre-compiled-binaries-any-os)
> and include the path to the `protoc` program under `bin` directory in your
> environment’s PATH variable. Copy the contents of the `include` directory
> somewhere as well, for example into `/usr/local/include/`. Next, to generate
> JavaScript code for gRPC-Web download the latest releases from [this
> repo](https://github.com/protocolbuffers/protobuf-javascript/releases) and
> [this repo](https://github.com/grpc/grpc-web/releases) and add the protoc
> plugins `protoc-gen-js` under `bin` directory and the `protoc-gen-grpc-web`
> program to your environment's PATH variable. Finally, run `task protoc` to
> generate new Protobuf, gRPC, and gRPC-Web service stubs.

To release a new DDA version run `task release`. It triggers a GitHub Actions
release workflow by pushing the current branch including a version tag with
annotated release notes read from the console.

For testing purposes run `task release-dry` to create a local release for which
release assets are deployed in the `dist` folder and Docker images are pushed
locally.

## License

Code and documentation copyright 2023 Siemens AG.

Code is licensed under the [MIT License](https://opensource.org/licenses/MIT).

Documentation is licensed under a
[Creative Commons Attribution-ShareAlike 4.0 International License](http://creativecommons.org/licenses/by-sa/4.0/).
