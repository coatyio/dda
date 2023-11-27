# Documentation of Data Distribution Agent (DDA)

This page is the central place for application-level developer documentation of
the Data Distribution Agent (DDA).

## Concepts and Quick Start

The [README](https://github.com/coatyio/dda#data-distribution-agent) contains an
overview of DDA concepts and a quick start guide on how to configure and run a
DDA as a sidecar.

## Developer Guide

The [DDA Developer Guide](./DEVGUIDE.md) describes how to use the public DDA
services and APIs in an application, either embedded as a programming library or
through a gRPC or gRPC-Web client.

## Reference Documentation

To integrate DDA as a library or to implement your own bindings for
communication protocols, storage, or consensus algorithms you can look up the
documentation of the DDA Golang module and its packages
[here](https://pkg.go.dev/github.com/coatyio/dda).

## Code Examples

The repository [dda-examples](https://github.com/coatyio/dda-examples) contains
a collection of ready-to-run best practice code examples demonstrating how to
use DDA as a sidecar or library in a decentralized desktop or web application.

## Coverage

[![Coverage Report](https://coatyio.github.io/dda/coverage-badge.svg)](https://coatyio.github.io/dda/coverage.html)

The test coverage report of the latest DDA release can be found
[here](https://coatyio.github.io/dda/coverage.html).

---

Copyright (c) 2023 Siemens AG. Documentation is licensed under a
[Creative Commons Attribution-ShareAlike 4.0 International License](http://creativecommons.org/licenses/by-sa/4.0/).
