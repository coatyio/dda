# DDA Developer Guide

This document targets developers who want to use data-centric services exposed
by a Data Distribution Agent (DDA) to realize distributed, decentralized, and
collaborative application scenarios.

We assume you have delved into the
[README](https://github.com/coatyio/dda#data-distribution-agent) before working
with this guide. The README contains a motivation and overview of DDA concepts,
and a quick start guide on how to configure and run a DDA sidecar.

This developer guide is accompanied by a collection of ready-to-run best
practice [code examples](https://github.com/coatyio/dda-examples) and complete
[reference documentation](https://pkg.go.dev/github.com/coatyio/dda).

The software design and coding principles on which DDA is based follow the MAYA
(Most Advanced. Yet Acceptable.) design strategy and the YAGNI (You Aren’t Gonna
Need It) principle.

## Table of Contents

* [Getting Started](#getting-started)
* [Encoding Application Data](#encoding-application-data)
* [Using DDA as a Sidecar](#using-dda-as-a-sidecar)
  * [Configuration](#configuration)
  * [gRPC Client API](#grpc-client-api)
  * [gRPC-Web Client API](#grpc-web-client-api)
* [Using DDA as a Library](#using-dda-as-a-library)
  * [Configure DDA Instance](#configure-dda-instance)
  * [Open and Close DDA Instance](#open-and-close-dda-instance)
  * [Invoke DDA Services](#invoke-dda-services)
* [License](#license)

## Getting Started

For application developers DDA functionality comes in two variants:

* as a sidecar that is attached to a primary application component
* as a programming library to be directly embedded into a primary application
  component

In both variants DDA exposes a collection of application-level peripheral
services to its primary application component, including data-centric
communication, distributed state synchronization, and local persistent storage.

Particular DDA peripheral services may require additional infrastructure
components to be set up. For example, the data-centric communication service
uses a configurable underlying publish-subscribe messaging protocol which may
depend on a message broker or router that must be running and reachable in your
network. If the default protocol MQTT 5 is configured, an MQTT 5 compatible
broker, such as [emqx](https://www.emqx.io/) or
[mosquitto](https://mosquitto.org/), must be made available.

## Encoding Application Data

Principally, application data transmitted to/received from a peripheral service
must be encoded/decoded in a binary serialization format by the application.
Typical examples include UTF-8 encoded byte arrays for textual representations,
such as plain strings or [JSON](https://www.json.org), and binary
representations, such as [Protobuf](https://protobuf.dev/) and
[MsgPack](https://msgpack.org/), for structured data.

An application may choose to use a uniform binary encoding for all services or
use specific ones for specific services. You may also use _different_ encodings
in different service invocation contexts, e.g. specific to a certain
communication Event/Action/Query type. Encodings may even be different for
request and response data in two-way communication patterns.

In addition, the optional `DataContentType` property of Event/Action/Query types
may indicate to application components the concrete RFC 2046 content type of
binary encoded data.

## Using DDA as a Sidecar

Although a DDA sidecar is typically attached to a single application component
co-located on the same host, it is technically feasible to share it with other
application components located on the same or even on different hosts.

A DDA sidecar may be run as a standalone platform-specific binary or as a Docker
image and is configured by a YAML configuration file deployed along with it. For
details, see [Quick Start](https://github.com/coatyio/dda#quick-start).

### Configuration

Each DDA sidecar running in your distributed application needs to be configured
regarding its identity, public Client APIs, and peripheral services. A default
YAML configuration file `dda.yaml` is part of all DDA sidecar
[deployments](https://github.com/coatyio/dda/releases) and serves as a fully
documented template to configure your application-specific sidecars.

### gRPC Client API

An application component can consume the co-located DDA sidecar services using
[gRPC](https://grpc.io/), a high performance, open source universal RPC
framework supporting a wide range of programming languages and platforms. For
browser clients [gRPC-Web](https://github.com/grpc/grpc-web), a JavaScript
implementation of gRPC is supported by DDA (see [next
section](#grpc-web-client-api)).

DDA sidecar services are exposed as gRPC services which are defined in a
declarative way using [Protocol
Buffers](https://developers.google.com/protocol-buffers) (aka Protobuf). The
services are run on a gRPC server which is hosted inside the DDA sidecar. To
make use of the services, you have to implement a gRPC client in your
application component that connects to the gRPC server and invokes the defined
Remote Procedure Calls (RPCs). As gRPC works across languages and platforms, you
can program application components in any supported language. Usually, this is
an easy undertaking as gRPC supports automatic generation of client stubs based
on the given Protobuf definitions of RPCs and messages.

> __Performance Tip__: A DDA sidecar's gRPC server supports connections over TCP
> or Unix domain sockets. You may configure Unix sockets in the server address
> for fast, reliable, stable and low latency communication between gRPC client
> and server co-located on the _same_ Linux or macOS machine. To use Unix
> sockets with a DDA sidecar hosted in a local container the configured socket
> file needs to be shared via a volume.

If you are new to gRPC or would like to learn more, we recommend reading the
following documentation:

* [Introduction to gRPC](https://grpc.io)
* [gRPC documentation and supported programming
  languages](https://grpc.io/docs/)
* [Protobuf developer
  guide](https://developers.google.com/protocol-buffers/docs/overview)
* [Protobuf language
  references](https://developers.google.com/protocol-buffers/docs/reference/overview)

To program a gRPC client for one of the DDA peripheral services, follow these
steps:

1. Locate the service-specific Protobuf definition file that comes along with
   the DDA sidecar [deployments](https://github.com/coatyio/dda/releases) on
   GitHub. Service protos are fully documented and include any gRPC specific
   details you need to know on the client side, including error status codes
   that an RPC invocation may yield.
2. Generate gRPC client stubs from service protos using the recommended
   language-specific tool/compiler, such as
   [protoc](https://grpc.io/docs/protoc-installation). Some programming
   languages, like Node.js, also support dynamic loading and use of service
   definitions at runtime.
3. Implement application component business logic by programming handler
   functions for individual service RPCs. In principle, application-specific
   data is passed in binary format to service RPCs using the Protobuf `bytes`
   scalar value type. Therefore, you must encode your data into a sequence of
   bytes on the sending side, and decode the `bytes` message field on the
   receiving side.

When programming a gRPC client follow these guidelines:

* Read the detailed documentation on individual RPCs and message types included
  in the service-specific Protobuf files (single source of truth).
* Set useful [deadlines/timeouts](https://grpc.io/blog/deadlines/) in your unary
  calls for how long you are willing to wait for a reply from the server.
* Cancel a server streaming call on the client side as soon as you no longer
  want to receive further responses.
* Handle potential errors/cancellations for your client calls. DDA peripheral
  services use the standard gRPC error model as described
  [here](https://www.grpc.io/docs/guides/error/). Specific errors returned by
  service RPCs are documented in the corresponding Protobuf files.
* Whenever a (response) subscription has been set up by the gRPC server, it
  sends a gRPC header with a custom metadata key `dda-suback` on the server
  streaming call to acknowledge that the subscription has been established. A
  client may await and read this header from the stream _before_ receiving data
  to prevent race conditions with subsequent code that indirectly triggers a
  remote publication on the subscription which can be lost if the subscription
  has not yet been fully established by the pub-sub infrastructure.
* To avoid connection breaks for long-lived server streaming RPCs consider
  setting an appropriate client-side keepalive time but avoid configuring the
  keepalive much below one minute to mitigate DDoS (for details see
  [here](https://grpc.io/docs/guides/keepalive/)). You may also configure
  server-side keepalive time in DDA `grpc` configuration options which defaults
  to 2 hours.
* For performance reasons, prefer Unix sockets over TCP when connecting your
  client to a DDA sidecar that is co-located on the same Linux or macOS machine.
  You can also use volume mounted Unix socket files if your DDA sidecar runs in
  a local container.

### gRPC-Web Client API

If your application component is running in a browser, you may simply use the
prebuilt and ready-to-use gRPC-Web client stubs available as JavaScript modules
in the DDA sidecar [deployments](https://github.com/coatyio/dda/releases).

Detailed documentation on the usage of gRPC-Web client stubs can be found
[here](https://github.com/grpc/grpc-web#3-write-your-js-client) and in the DDA
[light control example](https://github.com/coatyio/dda-examples).

The general guidelines mentioned in the [previous section](#grpc-client-api)
also apply to gRPC-Web client programming. In addition, please note the
following:

* The JavaScript client stubs use CommonJS style with additional `.d.ts`
  TypeScript typings.
* The client stubs depend on the npm packages `grpc-web` and `google-protobuf`.
* Fields of type `bytes` in Protobuf message types are mapped to client stub
  getter/setter functions that expect an argument of type `Uint8Array | string`.
  If a `string` is passed it __must__ be base64 encoded, as payloads are
  encoded/decoded as such by gRPC-Web. Prefer to pass `UInt8Array` values
  directly. For example, to get/set a plain string or a JavaScript object as
  parameters in an Action communication pattern:

  ```js
  // Serialize an Action params object.
  const a = new Action().setParams(new TextEncoder().encode(JSON.stringify(params)));

  // Deserialize an Action params object.
  JSON.parse(new TextDecoder("utf-8").decode(a.getParams()));
  ```

* For gRPC-Web server streaming calls ensure to close the client readable stream
  by invoking the `cancel` function when the final response has been received or
  when no more responses should be processed. Closing the stream ensures that an
  underlying HTTP/1.1 connection is also closed (see remarks below).
* It is advisable to set a
  [deadline](https://github.com/grpc/grpc-web#setting-deadline) on the client
  side in case responses arrive too late (see also
  [here](https://grpc.io/blog/deadlines/)).

> __Note__: The grpc-web protocol is not really scalable if your browser uses
> HTTP/1.1 connections because each unary or server streaming call is invoked on
> a separate connection. If you have multiple long-lived gRPC-Web requests with
> unlimited responses over time, you might run into an issue with browser's
> maximum HTTP connection limit (e.g. 6 per domain and 10 total in Chrome). In
> this case, further requests are blocked until some HTTP connections are
> available again. If requester and responder clients run in the same browser
> this may also lead to deadlocks as a responder can no longer publish its
> response if the limit is reached. Note that this limitation doesn't exist if
> the browser uses HTTP/2 as it supports multiplexing multiple grpc-web calls on
> the same connection. The DDA gRPC-Web sidecar service supports both HTTP/1.1
> and HTTP/2 connections established by browser clients.
>
> __Note__: Header metadata with dda-suback acknowledgment is not emitted until
> the server stream is closed. You cannot receive metadata as header data before
> any response data although the HTTP response header contains it. This is an
> issue with the official [grpc-web client
> library](https://github.com/grpc/grpc-web). If you want to make use of
> dda-suback metadata in your app, consider using the
> [@improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web) library
> as an alternative. It supports header metadata to be received before any
> response data.

## Using DDA as a Library

An application component written in [Go](https://go.dev/) may directly import
DDA as a Go module:

```sh
go get github.com/coatyio/dda
```

Reference documentation of the DDA module and its packages can be found
[here](https://pkg.go.dev/github.com/coatyio/dda).

### Configure DDA Instance

Before use each DDA instance embedded in an application component needs to be
configured programmatically regarding its identity, public Client APIs, and
peripheral services.

A set of hierarchical structure types that represent the DDA YAML configuration
(see file `dda.yaml` in the project root folder) is available in package
[`config`](https://pkg.go.dev/github.com/coatyio/dda/config). A populated
instance of the root-level configuration type named
[`Config`](https://pkg.go.dev/github.com/coatyio/dda/config#Config) must be
passed when creating a `Dda` instance with
[`New`](https://pkg.go.dev/github.com/coatyio/dda/dda#New). Note that the fields
of all configuration types are not documented in code but solely in the default
YAML configuration file (single source of truth).

```go
import (
    "github.com/coatyio/dda/config"
    "github.com/coatyio/dda/dda"
)

// Create DDA configuration with default values.
cfg := config.New()

// Populate configuration options with all public Client APIs disabled (exemplary).
cfg.Identity.Name = "DDA-Foo"
cfg.Cluster = "my-app"
cfg.Apis.Grpc.Disabled = true       // Do not expose gRPC API
cfg.Apis.GrpcWeb.Disabled = true    // Do not expose gRPC-Web API
cfg.Services.Com.Protocol = "mqtt5"
cfg.Services.Com.Url = "tcp://mqttbrokerhost:1883"  // MQTT 5 Broker must be available

// Create DDA instance with given configuration.
inst, err := dda.New(cfg)
if err != nil {
    // Handle error
}
```

> __TIP__: Instead of hardcoding configuring options you may prefer to deploy a
> DDA YAML configuration file along with your application component. Use the
> function
> [`ReadConfig`](https://pkg.go.dev/github.com/coatyio/dda/config#ReadConfig) to
> read this configuration file and pass the returned `Config` structure to
> `dda.New`.

### Open and Close DDA Instance

To invoke DDA services and to accept requests from enabled public Client APIs a
configured DDA instance must be opened first. Close any opened instance if it is
no longer needed. You may reopen it at a later time.

```go
import "time"

// Start all configured services and enabled public Client APIs.
err := inst.Open(10 * time.Second)
if err != nil {
    // Handle error
}

// Now DDA instance is ready for operation...

// Finally, stop all configured services and enabled public Client APIs.
inst.Close()
```

### Invoke DDA Services

To invoke methods on a specific DDA service refer to the `Api` interface
definition in the corresponding service subpackage under
[`services/<srvname>/api`](https://pkg.go.dev/github.com/coatyio/dda/services#section-directories).

For example, to use the data-centric communication service named `com` inspect
API methods under package
[`services/com/api`](https://pkg.go.dev/github.com/coatyio/dda/services/com/api#Api).

The following code fragments show in an exemplary way how to publish one-way
communication _Events_ and how to subscribe to them assuming a DDA instance
`inst` has already been opened. Two-way communication _Actions_ and _Querys_
work in much the same manner.

```go
import (
    "context"
    "fmt"
    "log"
    "strconv"
    "time"

    "github.com/coatyio/dda/services/com/api"
)

const announceEventType = "com.mycompany.myapp.announcement"

// CreateAnnouncement creates and returns an event that represents an
// announcement. The tuple (id, source) should be unique for each event.
// Event data is transmitted in UTF-8 encoded binary serialization format.
func CreateAnnouncement(id int) api.Event {
    return api.Event{
        Type:   announceEventType,
        Id:     strconv.Itoa(i),
        Source: inst.Identity().Id,
        Data:   []byte(fmt.Sprintf("Announcement #%d from %s", id, inst.Identity().Name)),
    }
}

// SendAnnouncements creates and publishes the given number of announcement
// events, one per second.
func SendAnnouncements(num int) {
    for id := 1; id <= num; id++ {
        if err := inst.PublishEvent(CreateAnnouncement(id)); err != nil {
            log.Printf("Failed to send announcement: %v", err)
            return
        }
        time.Sleep(time.Second)
    }
}

// ReceiveAnnouncements subscribes to announcement events and handles them until
// the given context is canceled.
func ReceiveAnnouncements(ctx context.Context) {
    evts, err := inst.SubscribeEvent(ctx, api.SubscriptionFilter{Type: announceEventType})
    if err != nil {
        log.Printf("Failed to subscribe announcement events: %v", err)
        return
    }

    for evt := range evts {
        log.Printf("Received announcement: %s", string(evt.Data))
    }
}

ctx, cancel := context.WithCancel(context.Background())

go ReceiveAnnouncements(ctx)
go SendAnnouncements(42)

// Wait for 30 sec receiving approx. 30 announcements.
<-time.After(30 * time.Second)

// Unsubscribe announcement events so that they are no longer received on the 
// evts channel which is being closed asynchronously.
// 
// If required, you may await closing before continuing. This ensures that the
// corresponding unsubscription operation on the DDA communication binding has
// fully completed.
cancel()
```

> __NOTE__: Usually, there is no need to invoke the _API methods_ `Open` or
> `Close` of a _service_ explicitly as these are automatically called whenever a
> DDA instance is opened or closed.
>
> __NOTE__: DDA Library provides its own logger in the
> [`plog`](https://pkg.go.dev/github.com/coatyio/dda/plog) package which outputs
> certain information and errors to the console. You may also use this logger
> for application logging or turn it off by calling `plog.Disable()` early in
> your main function.

When programming a DDA instance follow these guidelines:

* Read the detailed documentation on individual API methods included in the
  service-specific API interface definition (single source of truth).
* Set useful cancelations/timeouts on channel receive operations for how long
  you are willing to wait for data from a service.
* To stop receiving further incoming data in service invocations, such as
  `SubscribeEvent`, `PublishAction`, `SubscribeAction`, etc., cancel its
  context explicitly or specify a context with a deadline/timeout.
* Handle potential errors for your service invocations. Specific errors returned
  by service methods are documented in the corresponding API interface
  definitions.
* For security reasons and to keep footprint small, _disable_ any public Client
  API in the DDA configuration that is not needed by your application. Note that
  by default _all_ public client APIs are _enabled_ in the configuration.

## License

Code and documentation copyright 2023 Siemens AG.

Code is licensed under the [MIT License](https://opensource.org/licenses/MIT).

Documentation is licensed under a
[Creative Commons Attribution-ShareAlike 4.0 International License](http://creativecommons.org/licenses/by-sa/4.0/).
