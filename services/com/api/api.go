// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package api provides the communication API for data-centric message-driven
// communication among decoupled DDAs. The communication API provides
// data-centric communication patterns for one-way and two-way communication
// based on the publish-subscribe messaging paradigm. The communication API is
// implemented by communication protocol bindings that use specific underlying
// pub-sub messaging transport protocols, like MQTT, Zenoh, Kafka, and others.
// Structured Event, Action, and Query data is described in a common way
// adhering closely to the CloudEvents specification.
//
// In addition, package api also provides common functionality to be reused by
// communication binding implementations. For example, type Router and related
// types may be used to manage subscription filters and to lookup associated
// handlers.
package api

import (
	"context"
	"fmt"
	"time"

	"github.com/coatyio/dda/config"
)

// A Scope identifies one of the supported DDA services that uses pub-sub
// communication. Scope is used as part of publications and corresponding
// subscription filters to support isolated routing of service specific
// messages.
type Scope string

const (
	ScopeDef         Scope = ""    // Default scope (ScopeCom)
	ScopeCom         Scope = "com" // Communication scope
	ScopeState       Scope = "sta" // Distributed state management scope
	ScopeStore       Scope = "sto" // Local storage scope
	ScopeSideChannel Scope = "sdc" // Side channel scope
)

// ToScope converts a Scope string to a Scope.
func ToScope(scope string) (Scope, error) {
	switch scope {
	case string(ScopeDef):
		return ScopeCom, nil
	case string(ScopeCom):
		return ScopeCom, nil
	case string(ScopeState):
		return ScopeState, nil
	case string(ScopeStore):
		return ScopeStore, nil
	case string(ScopeSideChannel):
		return ScopeSideChannel, nil
	default:
		return "", fmt.Errorf("unknown scope %s", scope)
	}
}

// Api is an interface for data-centric message-driven communication among
// decoupled DDAs. The communication API provides data-centric patterns for
// one-way and two-way communication based on the publish-subscribe messaging
// paradigm. The communication API is implemented by communication protocol
// bindings that use specific underlying pub-sub messaging transport protocols,
// like MQTT, Zenoh, Kafka, and others. Structured Event, Action, and Query data
// is described in a common way adhering closely to the CloudEvents
// specification.
//
// Note that Api implementations are meant to be thread-safe and Api interface
// methods may be run on concurrent goroutines.
type Api interface {

	// Open asynchronously connects to the communication network of the
	// underlying protocol binding with the supplied configuration.
	//
	// Upon successful connection or if the binding has been opened already, the
	// returned error channel yields nil and is closed. If connection fails
	// eventually, or if the given timeout elapses before connection is up, the
	// returned error channel yields an error and is closed. Specify a zero
	// timeout to disable preliminary timeout behavior.
	Open(cfg *config.Config, timeout time.Duration) <-chan error

	// Close asynchronously disconnects from the communication network of the
	// underlying protocol binding previously opened.
	//
	// The returned done channel is closed upon disconnection or if the binding
	// is not yet open.
	Close() (done <-chan struct{})

	// PublishEvent transmits the given Event with the given optional scope.
	//
	// PublishEvent blocks waiting for an acknowledgment, if configured
	// accordingly. An error is returned if the Event cannot be transmitted, if
	// acknowledgement times out, or if the binding is not yet open.
	PublishEvent(event Event, scope ...Scope) error

	// SubscribeEvent receives Events published on the specified subscription
	// filter.
	//
	// SubscribeEvent blocks waiting for an acknowledgment, if configured
	// accordingly. An error is returned along with a nil channel if
	// subscription fails, if acknowledgement times out, or if the binding is
	// not yet open.
	//
	// To unsubscribe from receiving events, cancel the given context which
	// causes the close of the events channel asynchronously.
	SubscribeEvent(ctx context.Context, filter SubscriptionFilter) (events <-chan Event, err error)

	// PublishAction transmits the given Action with the given optional scope,
	// receiving ActionResults through the returned buffered results channel.
	//
	// PublishAction blocks waiting for acknowledgments, if configured
	// accordingly. An error is returned along with a nil channel if Action
	// cannot be transmitted, if acknowledgement times out, if ActionResults
	// cannot be received, or if the binding is not yet open.
	//
	// To unsubscribe from receiving results, cancel the given context which
	// causes the close of the results channel asynchronously.
	PublishAction(ctx context.Context, action Action, scope ...Scope) (results <-chan ActionResult, err error)

	// SubscribeAction receives Actions published on the specified subscription
	// filter, and provides a callback to be invoked to transmit back
	// ActionResults.
	//
	// SubscribeAction blocks waiting for an acknowledgment, if configured
	// accordingly. An error is returned along with a nil channel if
	// subscription fails, if acknowledgement times out, or if the binding is
	// not yet open.
	//
	// To unsubscribe from receiving actions, cancel the given context which
	// causes the close of the actions channel asynchronously.
	SubscribeAction(ctx context.Context, filter SubscriptionFilter) (actions <-chan ActionWithCallback, err error)

	// PublishQuery transmits the given Query with the given optional scope,
	// receiving QueryResults through the returned buffered results channel.
	//
	// PublishQuery blocks waiting for acknowledgments, if configured
	// accordingly. An error is returned along with a nil channel if Query
	// cannot be transmitted, if acknowledgement times out, if QueryResults
	// cannot be received, or if the binding is not yet open.
	//
	// To unsubscribe from receiving results, cancel the given context which
	// causes the close of the results channel asynchronously.
	PublishQuery(ctx context.Context, query Query, scope ...Scope) (results <-chan QueryResult, err error)

	// SubscribeQuery receives Querys published on the specified subscription
	// filter, and provides a callback to be invoked to transmit back
	// QueryResults.
	//
	// SubscribeQuery blocks waiting for an acknowledgment, if configured
	// accordingly. An error is returned along with a nil channel if
	// subscription fails, if acknowledgement times out, or if the binding is
	// not yet open.
	//
	// To unsubscribe from receiving queries, cancel the given context which
	// causes the close of the queries channel asynchronously.
	SubscribeQuery(ctx context.Context, filter SubscriptionFilter) (queries <-chan QueryWithCallback, err error)
}

// A SubscriptionFilter defines the context that determines which publications
// should be transmitted to a subscriber.
type SubscriptionFilter struct {

	// Scope of the Event, Action, or Query with respect to the DDA service that
	// triggers it (optional).
	//
	// If not present or an empty string, the default scope "com" is used.
	Scope Scope

	// Type of Event, Action, or Query to be filtered (required).
	//
	// Must be a non-empty string consisting of lower-case ASCII letters ('a' to
	// 'z'), upper-case ASCII letters ('A' to 'Z'), ASCII digits ('0' to '9'),
	// ASCII dot ('.'), ASCII hyphen (-), or ASCII underscore (_).
	Type string

	// Name to be used for a shared subscription (optional).
	//
	// A shared subscription is not routed to all subscribers specifying the
	// same Scope, Type, and Share, but only to one of these. Shared
	// subscriptions may be used to load balance published tasks so as to
	// distribute workload evenly among a set of subscribers. Another use case
	// is high availability through redundancy where a secondary subscribers
	// takes over published tasks if the primary subscriber is no longer
	// reachable (hot standby). Typically, shared subscriptions are used with
	// the Action pattern.
	//
	// A published Event, Action, or Query is matching a shared subscription if
	// it provides the same Scope and Type. If multiple shared subscriptions
	// with different Share names but the same Scope and Type match such a
	// publication, it will be routed to one (and only one) in each Share group.
	//
	// If non-empty, must consist of lower-case ASCII letters ('a' to 'z'),
	// upper-case ASCII letters ('A' to 'Z'), ASCII digits ('0' to '9'), ASCII
	// dot ('.'), ASCII hyphen (-), or ASCII underscore (_).
	//
	// If not present or an empty string, the related subscription is not
	// shared.
	Share string
}

// Event is a structure expressing an occurrence and its context. An event may
// occur due to a raised or observed signal, a state change, an elapsed timer,
// an observed or taken measurement, or any other announcement or activity. An
// Event is routed from an event producer (source) to interested event consumers
// using pub-sub messaging.
type Event struct {

	// Type of event related to the originating occurrence (required).
	//
	// Type is used as a subscription filter for routing the event to consumers
	// via pub-sub messaging. Must be a non-empty string consisting of
	// lower-case ASCII letters ('a' to 'z'), upper-case ASCII letters ('A' to
	// 'Z'), ASCII digits ('0' to '9'), ASCII dot ('.'), ASCII hyphen (-), or
	// ASCII underscore (_).
	//
	// Follow a consistent naming convention for types throughout an application
	// to avoid naming collisions. For example, Type could use Reverse Domain
	// Name Notation (com.mycompany.myapp.mytype) or some other hierarchical
	// naming pattern with some levels in the hierarchy separated by dots,
	// hyphens, or underscores.
	Type string

	// Identifies the event (required).
	//
	// Id must be non-empty and unique within the scope of the producer.
	// Producers must ensure that (Source, Id) is unique for each distinct
	// event. Consumers may assume that events with identical Source and Id are
	// duplicates.
	//
	// Typically, Id is a UUID or a counter maintained by the producer.
	Id string

	// Identifies the context in which the event occurred (required).
	//
	// An event source is defined by the event producer. Producers must ensure
	// that (Source, Id) is unique for each distinct event. Source must be
	// non-empty.
	//
	// Typically, Source may be a URI describing the organization publishing the
	// event or the process that generates the event.
	Source string

	// Timestamp when the occurrence happened or when the event data has been
	// generated (optional).
	//
	// If present, must adhere to the format specified in [RFC 3339]. An empty
	// string value indicates that a timestamp is not available or needed.
	//
	// [RFC 3339]: https://www.rfc-editor.org/rfc/rfc3339
	Time string

	// Domain-specific payload information about the occurrence (required).
	//
	// Encoding and decoding of the transmitted binary data is left to the user
	// of the API interface. Any binary serialization format can be used.
	Data []byte

	// Content type of Data value (optional).
	//
	// If present, it must adhere to the format specified in [RFC 2046]. An
	// empty string value indicates that a content type is implied by the
	// application.
	//
	// [RFC 2046]: https://www.rfc-editor.org/rfc/rfc2046
	DataContentType string
}

// Action is a structure expressing an action, command, or operation to be
// carried out by interested action consumers. An Action is routed from an
// action invoker to interested action consumers using pub-sub messaging.
type Action struct {

	// Type of action, command or operation to be performed (required).
	//
	// Type is used as a subscription filter for routing the action to consumers
	// via pub-sub messaging. Must be a non-empty string consisting of
	// lower-case ASCII letters ('a' to 'z'), upper-case ASCII letters ('A' to
	// 'Z'), ASCII digits ('0' to '9'), ASCII dot ('.'), ASCII hyphen (-), or
	// ASCII underscore (_).
	//
	// Follow a consistent naming convention for types throughout an application
	// to avoid naming collisions. For example, Type could use Reverse Domain
	// Name Notation (com.mycompany.myapp.mytype) or some other hierarchical
	// naming pattern with some levels in the hierarchy separated by dots,
	// hyphens, or underscores.
	Type string

	// Identifies the action (required).
	//
	// Id must be non-empty and unique within the scope of the action invoker.
	// Invokers must ensure that (Source, Id) is unique for each distinct
	// action. Consumers may assume that actions with identical Source and Id
	// are duplicates.
	//
	// Typically, Id is a UUID or a counter maintained by the invoker.
	Id string

	// Identifies the context in which the action is invoked (required).
	//
	// An action source is defined by the action invoker. Invokers must ensure
	// that (Source, Id) is unique for each distinct action. Source must be
	// non-empty.
	//
	// Typically, Source may be a URI describing the organization publishing the
	// action or the process that invokes the action.
	Source string

	// Data describing the parameters of the action (optional).
	//
	// Encoding and decoding of the transmitted binary data is left to the user
	// of the API interface. Any binary serialization format can be used.
	Params []byte

	// Content type of Params value (optional).
	//
	// If present, it must adhere to the format specified in [RFC 2046]. An
	// empty string value indicates that a content type is implied by the
	// application.
	//
	// [RFC 2046]: https://www.rfc-editor.org/rfc/rfc2046
	DataContentType string
}

// ActionResult is a structure containing resulting information returned to the
// invoker of an Action. Each interested action consumer may transmit its own
// action result(s) independently of the others. Multiple ActionResults over
// time may be generated by a consumer for a single Action to transmit
// progressive series of results.
type ActionResult struct {

	// Identifies the context, in which the action is executed (required).
	//
	// Typically, Context may be a URI describing the organization consuming the
	// action or the process that carries out the action.
	Context string

	// Resulting data to be returned to the action invoker (required).
	//
	// Note that errors occurring while processing an action are also encoded as
	// result binary data in a domain-specific way.
	//
	// Encoding and decoding of the transmitted binary data is left to the user
	// of the API interface. Any binary serialization format can be used.
	Data []byte

	// Content type of Data value (optional).
	//
	// If present, it must adhere to the format specified in [RFC 2046]. An
	// empty string value indicates that a content type is implied by the
	// application.
	//
	// [RFC 2046]: https://www.rfc-editor.org/rfc/rfc2046
	DataContentType string

	// The sequence number of a multi-result response (required for progressive
	// responses only).
	//
	// A zero value or -1 indicates a single result. If multiple ActionResults
	// are to be returned, the sequence number is 1 for the first result and
	// incremented by one with each newly generated result. If sequence number
	// overflows its maximum value 9223372036854775807, the next value should
	// revert to 1. A final result should be indicated by using the additive
	// inverse of the generated sequence number.
	//
	// A zero or negative sequence number indicates that no more results will be
	// published for the correlated action after the given one.
	SequenceNumber int64
}

// ActionWithCallback embeds an Action with an associated callback function to
// be invoked whenever an ActionResult should be transmitted back to the
// publisher of the Action.
type ActionWithCallback struct {

	// Action associated with response callback function.
	Action

	// Callback when invoked transmits an ActionResult to the publisher of the
	// correlated Action.
	//
	// An error is returned if ActionResult cannot be transmitted, or if the
	// binding is not yet opened.
	Callback ActionCallback
}

// ActionCallback is invoked by subscribers to transmit an ActionResult back to
// the publisher.
type ActionCallback func(result ActionResult) error

// Query is a structure expressing a query to be answered by interested query
// consumers. A Query is routed from a querier to interested query consumers
// using pub-sub messaging.
type Query struct {

	// Type of query indicating intent or desired result (required).
	//
	// Type is used as a subscription filter for routing the query to consumers
	// via pub-sub messaging. Must be a non-empty string consisting of
	// lower-case ASCII letters ('a' to 'z'), upper-case ASCII letters ('A' to
	// 'Z'), ASCII digits ('0' to '9'), ASCII dot ('.'), ASCII hyphen (-), or
	// ASCII underscore (_).
	//
	// Follow a consistent naming convention for types throughout an application
	// to avoid naming collisions. For example, Type could use Reverse Domain
	// Name Notation (com.mycompany.myapp.mytype) or some other hierarchical
	// naming pattern with some levels in the hierarchy separated by dots,
	// hyphens, or underscores.
	Type string

	// Identifies the query (required).
	//
	// Id must be non-empty and unique within the scope of the querier. Queriers
	// must ensure that (Source, Id) is unique for each distinct query.
	// Consumers may assume that queries with identical Source and Id are
	// duplicates.
	//
	// Typically, Id is a UUID or a counter maintained by the querier.
	Id string

	// Identifies the context in which the query is posed (required).
	//
	// A query source is defined by the querier. Queriers must ensure that
	// (Source, Id) is unique for each distinct query. Source must be non-empty.
	//
	// Typically, Source may be a URI describing the organization publishing the
	// query or the process that poses the query.
	Source string

	// Query data represented as indicated by Format (required).
	//
	// Encoding and decoding of the transmitted binary data is left to the user
	// of the API interface. Any binary serialization format can be used.
	Data []byte

	// Content type of Data value (optional).
	//
	// If present, it must adhere to the format specified in [RFC 2046]. An
	// empty string value indicates that a content type is implied by the
	// application.
	//
	// The context type should represent the query language/format. For example,
	// a GraphQL query should use "application/graphql" and a SPARQL query
	// should use "application/sparql-query".
	//
	// [RFC 2046]: https://www.rfc-editor.org/rfc/rfc2046
	DataContentType string
}

// QueryResult is a structure containing resulting information returned to the
// querier. Each interested query consumer may transmit its own query result(s)
// independently of the others. Multiple QueryResults over time may be generated
// by a consumer for a single Query to transmit live query results whenever the
// query yields new results due to update operations on the database.
type QueryResult struct {

	// Identifies the context, in which the query is executed (required).
	//
	// Typically, Context may be a URI describing the organization consuming the
	// query or the process that retrieves query result data.
	Context string

	// Query result data returned to the querier (required).
	//
	// Encoding and decoding of the transmitted binary data is left to the user
	// of the API interface. Any binary serialization format can be used.
	Data []byte

	// Content type of Data value (optional).
	//
	// If present, it must adhere to the format specified in [RFC 2046]. An
	// empty string value indicates that a content type is implied by the
	// application.
	//
	// If present, use MIME Content Types to specify the query result format.
	// For example, use "application/sql" for a SQL query result,
	// "application/graphql" for a GraphQL query result,
	// "application/sparql-results+json" for a SPARQL query result encoded in
	// JSON.
	//
	// [RFC 2046]: https://www.rfc-editor.org/rfc/rfc2046
	DataContentType string

	// The sequence number of a multi-result live query (required for live query
	// responses only).
	//
	// A zero value or -1 indicates a single result. If multiple QueryResults
	// are to be returned, the sequence number is 1 for the first result and
	// incremented by one with each newly generated result. If sequence number
	// overflows its maximum value 9223372036854775807, the next value should
	// revert to 1. A final result should be indicated by using the additive
	// inverse of the generated sequence number.
	//
	// A zero or negative sequence number indicates that no more results will be
	// published for the correlated action after the given one.
	SequenceNumber int64
}

// QueryWithCallback embeds a Query with an associated callback function to be
// invoked whenever a QueryResult should be transmitted back to the publisher of
// the Query.
type QueryWithCallback struct {

	// Query associated with response callback function.
	Query

	// Callback when invoked transmits a QueryResult to the publisher of the
	// correlated Query.
	//
	// An error is returned if QueryResult cannot be transmitted, or if the
	// binding is not yet opened.
	Callback QueryCallback
}

// QueryCallback is invoked by subscribers to transmit a QueryResult back to the
// publisher.
type QueryCallback func(result QueryResult) error
