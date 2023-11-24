// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package mqtt5 provides a communication protocol binding implementation for
// MQTT v5 transport protocol.
package mqtt5

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services"
	"github.com/coatyio/dda/services/com/api"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/google/uuid"
)

const (
	disconnectTimeout = 1 * time.Second // time to wait for disconnect to complete
	pubSubAckTimeout  = 1 * time.Second // time to wait for acknowledgments with QoS 1 and 2
)

const (
	levelShare        = "$share" // Shared topic prefix
	levelEvent        = "evt"    // Event topic level
	levelAction       = "act"    // Action topic level
	levelActionResult = "acr"    // ActionResult topic level
	levelQuery        = "qry"    // Query topic level
	levelQueryResult  = "qrr"    // QueryResult topic level
)

const (
	userPropId              = "id" // user property Id
	userPropSource          = "sr" // user property Source
	userPropTime            = "tm" // user property Time
	userPropContext         = "ct" // user property Context
	userPropSequenceNumber  = "sq" // user property SequenceNumber
	userPropDataContentType = "dt" // user property DataContentType
)

var strictClientIdRegex = regexp.MustCompile("[^0-9a-zA-Z]")

type mqttRouteFilter = api.RouteFilter[string]

// Mqtt5Binding realizes a communication protocol binding for MQTT v5 by
// implementing interface api.Api.
type Mqtt5Binding struct {
	mu                 sync.RWMutex // protects following fields
	eventRouter        *api.Router[api.Event, string]
	actionRouter       *api.Router[api.ActionWithCallback, string]
	actionResultRouter *api.Router[api.ActionResult, string]
	queryRouter        *api.Router[api.QueryWithCallback, string]
	queryResultRouter  *api.Router[api.QueryResult, string]
	clientId           string
	conn               *autopaho.ConnectionManager
	qos                byte   // used for all publications and subscriptions
	noLocal            bool   // used for all subscriptions
	cluster            string // used as topic root field
	responseId         string // unique ID in response topics
	sharedSubAvailable bool   // are shared subscriptions supported by broker
}

// ClientId returns the MQTT client ID used to connect to the broker (exposed
// for testing purposes).
func (b *Mqtt5Binding) ClientId() string {
	return b.clientId
}

func (b *Mqtt5Binding) Open(cfg *config.Config, timeout time.Duration) <-chan error {
	ch := make(chan error, 1)
	defer close(ch)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil {
		ch <- nil
		return ch
	}

	b.clientId = b.getClientId(cfg)
	b.cluster = cfg.Cluster
	b.responseId = cfg.Identity.Id
	b.noLocal = cfg.Services.Com.Opts["noLocal"] == true

	b.eventRouter = api.NewRouter[api.Event, string]()
	b.actionRouter = api.NewRouter[api.ActionWithCallback, string]()
	b.actionResultRouter = api.NewRouter[api.ActionResult, string]()
	b.queryRouter = api.NewRouter[api.QueryWithCallback, string]()
	b.queryResultRouter = api.NewRouter[api.QueryResult, string]()

	connackChan := make(chan *paho.Connack, 1)
	ccfg, err := b.getClientConfig(cfg, connackChan)
	if err != nil {
		ch <- err
		return ch
	}

	plog.Printf("Open MQTT5 binding connecting to %s...\n", ccfg.BrokerUrls[0])

	// Note that we cannot use a context.WithTimeout for NewConnection as
	// autopaho disconnects as soon as the passed context is canceled, i.e. when
	// the timeout elapses, even if the connection is already up!
	//
	// Note that NewConnection never returns an error in the currently used
	// implementation.
	conn, err := autopaho.NewConnection(context.Background(), *ccfg)
	if err != nil {
		ch <- err
		return ch
	}

	// If a broker URL has an unsupported schema or if broker connection cannot
	// be established paho.golang simply tries the next URL, endlessly. In this
	// case AwaitConnection will block until the passed timeout elapses.
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := conn.AwaitConnection(ctx); err != nil {
		_ = conn.Disconnect(context.Background())
		ch <- services.NewRetryableError(err)
		return ch
	}

	b.sharedSubAvailable = (<-connackChan).Properties.SharedSubAvailable
	b.conn = conn

	return ch
}

func (b *Mqtt5Binding) Close() (done <-chan struct{}) {
	ch := make(chan struct{})
	defer close(ch)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn == nil {
		return ch
	}

	ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
	defer cancel()
	_ = b.conn.Disconnect(ctx)

	b.conn = nil

	return ch
}

func (b *Mqtt5Binding) PublishEvent(event api.Event, scope ...api.Scope) error {
	if err := b.validatePatternTypeIdSource("Event", event.Type, event.Id, event.Source); err != nil {
		return err
	}

	scp := api.ScopeDef
	if len(scope) > 0 {
		scp = scope[0]
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return fmt.Errorf("PublishEvent %+v failed as binding is not yet open", event)
	}

	topic, _ := b.topicWithLevels("", scp, levelEvent, event.Type)
	return b.publish(topic, event.Data, paho.UserProperties{
		{Key: userPropId, Value: event.Id},
		{Key: userPropSource, Value: event.Source},
		{Key: userPropTime, Value: event.Time},
		{Key: userPropDataContentType, Value: event.DataContentType},
	}, "", nil)
}

func (b *Mqtt5Binding) SubscribeEvent(ctx context.Context, filter api.SubscriptionFilter) (events <-chan api.Event, err error) {
	if err := b.validatePatternFilter("Event", filter); err != nil {
		return nil, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return nil, fmt.Errorf("SubscribeEvent failed as binding is not yet open")
	}

	pubTopic, subTopic := b.topicWithLevels(filter.Share, filter.Scope, levelEvent, filter.Type)
	routeFilter := mqttRouteFilter{Topic: pubTopic}
	rc, err := b.eventRouter.Add(ctx, routeFilter,
		func() error { return b.subscribe(subTopic) },
		func() error { return nil },
		func() error { return b.unsubscribe(subTopic) },
	)
	if err != nil {
		return nil, err
	}
	return rc.ReceiveChan, nil
}

func (b *Mqtt5Binding) PublishAction(ctx context.Context, action api.Action, scope ...api.Scope) (results <-chan api.ActionResult, err error) {
	if err := b.validatePatternTypeIdSource("Action", action.Type, action.Id, action.Source); err != nil {
		return nil, err
	}

	scp := api.ScopeDef
	if len(scope) > 0 {
		scp = scope[0]
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return nil, fmt.Errorf("PublishAction %+v failed as binding is not yet open", action)
	}

	pubTopic, _ := b.topicWithLevels("", scp, levelAction, action.Type)
	pubResTopic, _ := b.topicWithLevels("", scp, levelActionResult, action.Type)
	responseTopic, correlationId := b.responseInfo(pubResTopic)
	routeFilter := mqttRouteFilter{Topic: responseTopic, CorrelationId: correlationId}
	rc, err := b.actionResultRouter.Add(ctx, routeFilter,
		func() error { return b.subscribe(responseTopic) },
		func() error {
			return b.publish(pubTopic, action.Params, paho.UserProperties{
				{Key: userPropId, Value: action.Id},
				{Key: userPropSource, Value: action.Source},
				{Key: userPropDataContentType, Value: action.DataContentType},
			}, responseTopic, []byte(correlationId))
		},
		func() error { return b.unsubscribe(responseTopic) },
	)
	if err != nil {
		return nil, err
	}
	return rc.ReceiveChan, nil
}

func (b *Mqtt5Binding) publishActionResult(result api.ActionResult, responseTopic string, correlationId []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return fmt.Errorf("publishActionResult %+v failed as binding is not yet open", result)
	}

	return b.publish(responseTopic, result.Data, paho.UserProperties{
		{Key: userPropContext, Value: result.Context},
		{Key: userPropDataContentType, Value: result.DataContentType},
		{Key: userPropSequenceNumber, Value: strconv.FormatInt(result.SequenceNumber, 10)},
	}, "", []byte(correlationId))
}

func (b *Mqtt5Binding) SubscribeAction(ctx context.Context, filter api.SubscriptionFilter) (actions <-chan api.ActionWithCallback, err error) {
	if err := b.validatePatternFilter("Action", filter); err != nil {
		return nil, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return nil, fmt.Errorf("SubscribeAction failed as binding is not yet open")
	}

	pubTopic, subTopic := b.topicWithLevels(filter.Share, filter.Scope, levelAction, filter.Type)
	routeFilter := mqttRouteFilter{Topic: pubTopic}
	rc, err := b.actionRouter.Add(ctx, routeFilter,
		func() error { return b.subscribe(subTopic) },
		func() error { return nil },
		func() error { return b.unsubscribe(subTopic) },
	)
	if err != nil {
		return nil, err
	}
	return rc.ReceiveChan, nil
}

func (b *Mqtt5Binding) PublishQuery(ctx context.Context, query api.Query, scope ...api.Scope) (results <-chan api.QueryResult, err error) {
	if err := b.validatePatternTypeIdSource("Query", query.Type, query.Id, query.Source); err != nil {
		return nil, err
	}

	scp := api.ScopeDef
	if len(scope) > 0 {
		scp = scope[0]
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return nil, fmt.Errorf("PublishQuery %+v failed as binding is not yet open", query)
	}

	pubTopic, _ := b.topicWithLevels("", scp, levelQuery, query.Type)
	pubResTopic, _ := b.topicWithLevels("", scp, levelQueryResult, query.Type)
	responseTopic, correlationId := b.responseInfo(pubResTopic)
	routeFilter := mqttRouteFilter{Topic: responseTopic, CorrelationId: correlationId}
	rc, err := b.queryResultRouter.Add(ctx, routeFilter,
		func() error { return b.subscribe(responseTopic) },
		func() error {
			return b.publish(pubTopic, query.Data, paho.UserProperties{
				{Key: userPropId, Value: query.Id},
				{Key: userPropSource, Value: query.Source},
				{Key: userPropDataContentType, Value: query.DataContentType},
			}, responseTopic, []byte(correlationId))
		},
		func() error { return b.unsubscribe(responseTopic) },
	)
	if err != nil {
		return nil, err
	}
	return rc.ReceiveChan, nil
}

func (b *Mqtt5Binding) publishQueryResult(result api.QueryResult, responseTopic string, correlationId []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return fmt.Errorf("publishQueryResult %+v failed as binding is not yet open", result)
	}

	return b.publish(responseTopic, result.Data, paho.UserProperties{
		{Key: userPropContext, Value: result.Context},
		{Key: userPropDataContentType, Value: result.DataContentType},
		{Key: userPropSequenceNumber, Value: strconv.FormatInt(result.SequenceNumber, 10)},
	}, "", []byte(correlationId))
}

func (b *Mqtt5Binding) SubscribeQuery(ctx context.Context, filter api.SubscriptionFilter) (queries <-chan api.QueryWithCallback, err error) {
	if err := b.validatePatternFilter("Query", filter); err != nil {
		return nil, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return nil, fmt.Errorf("SubscribeQuery failed as binding is not yet open")
	}

	pubTopic, subTopic := b.topicWithLevels(filter.Share, filter.Scope, levelQuery, filter.Type)
	routeFilter := mqttRouteFilter{Topic: pubTopic}
	rc, err := b.queryRouter.Add(ctx, routeFilter,
		func() error { return b.subscribe(subTopic) },
		func() error { return nil },
		func() error { return b.unsubscribe(subTopic) },
	)
	if err != nil {
		return nil, err
	}
	return rc.ReceiveChan, nil
}

func (b *Mqtt5Binding) getClientConfig(cfg *config.Config, connackChan chan<- *paho.Connack) (*autopaho.ClientConfig, error) {
	var comCfg = cfg.Services.Com
	var clientConfig autopaho.ClientConfig

	clientConfig = autopaho.ClientConfig{
		OnConnectionUp: func(conn *autopaho.ConnectionManager, ack *paho.Connack) {
			if connackChan != nil {
				clientConfig.Debug.Printf("broker connection up\n")
				defer close(connackChan)
				connackChan <- ack
				connackChan = nil
			} else {
				clientConfig.Debug.Printf("broker reconnection up\n")
				b.resubscribe()
			}
		},
		OnConnectError: func(err error) { clientConfig.Debug.Printf("broker connection error: %v\n", err) },
		Debug:          paho.NOOPLogger{},
		ClientConfig: paho.ClientConfig{
			ClientID:      b.clientId,
			OnClientError: func(err error) { clientConfig.Debug.Printf("client error: %v\n", err) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					clientConfig.Debug.Printf("broker requested disconnect: %s\n", d.Properties.ReasonString)
				} else {
					clientConfig.Debug.Printf("broker requested disconnect with reason code: %d\n", d.ReasonCode)
				}
			},
			Router: paho.NewSingleHandlerRouter(b.handle),
		},
	}

	if comCfg.Opts["debug"] == true && plog.Enabled() {
		// Inject paho and autopaho log output into standard logger.
		clientConfig.Debug = plog.WithPrefix("autopaho ")
		clientConfig.PahoDebug = plog.WithPrefix("paho ")
	}

	clientConfig.SetConnectPacketConfigurator(func(c *paho.Connect) *paho.Connect {
		// As long as client persistence is not supported in paho, a Connection
		// should start a new session on both client and server. See comment
		// https://github.com/eclipse/paho.golang/blob/d63b3b28d25ff73076c8846c92c4d062503e646e/autopaho/auto.go#L125
		c.CleanStart = true
		return c
	})

	sUrl := comCfg.Url
	if sUrl == "" {
		sUrl = "tcp://localhost:1883"
	}
	if brokerUrl, err := url.Parse(sUrl); err != nil {
		return nil, fmt.Errorf("invalid 'services.com.url' in DDA configuration: %w", err)
	} else {
		clientConfig.BrokerUrls = []*url.URL{brokerUrl}
	}

	if comCfg.Auth.Password != "" {
		// Note: MQTT version 5 of the protocol allows the sending of a Password
		// with no User Name, where MQTT v3.1.1 did not. This reflects the
		// common use of Password for credentials other than a password.
		clientConfig.SetUsernamePassword(comCfg.Auth.Username, []byte(comCfg.Auth.Password))
	} else {
		clientConfig.ResetUsernamePassword()
	}

	switch comCfg.Auth.Method {
	case "none", "":
	case "tls":
		cert, err := tls.LoadX509KeyPair(comCfg.Auth.Cert, comCfg.Auth.Key)
		if err != nil {
			return nil, fmt.Errorf("invalid or missing PEM file in DDA configuration under 'services.com.auth.cert/.key' : %w", err)
		}
		//#nosec G402 -- Default for configuration option Verify is true
		clientConfig.TlsCfg = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: !comCfg.Auth.Verify,
		}
	default:
		return nil, fmt.Errorf("unsupported 'services.com.auth.method' %s in DDA configuration", comCfg.Auth.Method)
	}

	switch v := comCfg.Opts["keepAlive"].(type) {
	case int:
		clientConfig.KeepAlive = uint16(v)
	default:
		clientConfig.KeepAlive = 30
	}

	switch v := comCfg.Opts["qos"].(type) {
	case int:
		b.qos = byte(v)
	default:
		b.qos = byte(0)
	}

	switch v := comCfg.Opts["connectRetryDelay"].(type) {
	case int:
		clientConfig.ConnectRetryDelay = time.Duration(v) * time.Millisecond
	default:
		clientConfig.ConnectRetryDelay = 1000 * time.Millisecond
	}

	switch v := comCfg.Opts["connectTimeout"].(type) {
	case int:
		clientConfig.ConnectTimeout = time.Duration(v) * time.Millisecond
	default:
		clientConfig.ConnectTimeout = 10000 * time.Millisecond
	}

	return &clientConfig, nil
}

func (b *Mqtt5Binding) getClientId(cfg *config.Config) string {
	clientId := cfg.Identity.Name + cfg.Identity.Id
	if cfg.Services.Com.Opts["strictClientId"] == true {
		// MQTT Version 5.0 Specification: [MQTT-3.1.3-5]
		//
		// The Server MUST allow ClientID’s which are between 1 and 23 UTF-8
		// encoded bytes in length, and that contain only the characters
		// "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ".
		//
		// The Server MAY allow ClientID’s that contain more than 23 encoded
		// bytes. The Server MAY allow ClientID’s that contain characters not
		// included in the list given above.
		clientId = strictClientIdRegex.ReplaceAllLiteralString(clientId, "0")
		if len(clientId) > 23 {
			clientId = clientId[0:23]
		}
	}
	return clientId
}

func (b *Mqtt5Binding) validatePatternTypeIdSource(pat string, typ string, id string, source string) error {
	if err := config.ValidateName(typ, pat, "Type"); err != nil {
		return err
	}
	if err := config.ValidateNonEmpty(id, pat, "Id"); err != nil {
		return err
	}
	if err := config.ValidateNonEmpty(source, pat, "Source"); err != nil {
		return err
	}
	return nil
}

func (b *Mqtt5Binding) validatePatternFilter(pat string, filter api.SubscriptionFilter) error {
	if err := config.ValidateName(filter.Type, pat, "Type"); err != nil {
		return err
	}
	if filter.Share != "" {
		if !b.sharedSubAvailable {
			return fmt.Errorf("shared subscriptions are not supported by the MQTT 5 broker")
		}
		if err := config.ValidateName(filter.Share, pat, "Share"); err != nil {
			return err
		}
	}
	return nil
}

func (b *Mqtt5Binding) topicWithLevels(share string, scope api.Scope, levels ...string) (pubTopic string, subTopic string) {
	if scope == api.ScopeDef {
		scope = api.ScopeCom
	}
	pubTopic = fmt.Sprintf("%s/%s/%s", b.cluster, scope, strings.Join(levels, "/"))
	subTopic = pubTopic
	if share != "" {
		subTopic = fmt.Sprintf("%s/%s/%s/%s/%s", levelShare, share, b.cluster, scope, strings.Join(levels, "/"))
	}
	return
}

func (b *Mqtt5Binding) createPubSubContext() (context.Context, context.CancelFunc) {
	var cancel context.CancelFunc = func() {}
	ctx := context.Background()
	if b.qos != 0 {
		ctx, cancel = context.WithTimeout(ctx, pubSubAckTimeout)
	}
	return ctx, cancel
}

func (b *Mqtt5Binding) responseInfo(topic string) (string, string) {
	return fmt.Sprintf("%s/%s", topic, b.responseId), uuid.NewString()
}

func (b *Mqtt5Binding) publish(topic string, payload []byte, userProps paho.UserProperties, responseTopic string, correlationId []byte) error {
	ctx, cancel := b.createPubSubContext()
	defer cancel()

	props := &paho.PublishProperties{
		// Note: PayloadFormat 1 means payload must conform to UTF-8 string
		// encoding (broker must check it!). As any binary data can be sent,
		// do not use it!
		//
		// PayloadFormat: paho.Byte(1), ContentType:   "text/plain", //
		// "application/json",

		User: userProps,
	}

	if responseTopic != "" {
		props.ResponseTopic = responseTopic
	}
	if correlationId != nil {
		props.CorrelationData = correlationId
	}

	p := &paho.Publish{
		QoS:        b.qos,
		Topic:      topic,
		Payload:    payload,
		Properties: props,
	}

	if _, err := b.conn.Publish(ctx, p); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

func (b *Mqtt5Binding) subscribe(topics ...string) error {
	if len(topics) == 0 {
		return nil
	}

	ctx, cancel := b.createPubSubContext()
	defer cancel()

	subs := make([]paho.SubscribeOptions, 0, len(topics))
	for _, topic := range topics {
		subs = append(subs, paho.SubscribeOptions{
			Topic: topic,
			QoS:   b.qos,

			// It is a Protocol Error to set the No Local bit to 1 on a
			// Shared Subscription [MQTT-3.8.3-4].
			NoLocal: b.noLocal && !strings.HasPrefix(topic, levelShare),
		})
	}
	s := &paho.Subscribe{Subscriptions: subs}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.conn == nil {
		return fmt.Errorf("subscribe failed as binding is not yet open")
	}

	if _, err := b.conn.Subscribe(ctx, s); err != nil {
		return services.RetryableErrorf("subscribe %v failed: %w", topics, err)
	}
	return nil
}

func (b *Mqtt5Binding) unsubscribe(topics ...string) error {
	if len(topics) == 0 {
		return nil
	}

	ctx, cancel := b.createPubSubContext()
	defer cancel()

	u := &paho.Unsubscribe{Topics: topics}

	// Trying to acquire read lock only fails if unsubscribe is invoked in a
	// context that has already acquired a write lock, i.e. within the Close
	// method invoking router.Clear. In this case, unsubscribe would deadlock
	// when acquiring a recursive read lock. As the write lock is already
	// acquired we can safely continue without read locking. All other
	// invocations of unsubscribe are in a user code context when calling an
	// UnsubscribeBindungFunc, so acquiring the read lock won't fail.
	if b.mu.TryRLock() {
		defer b.mu.RUnlock()
	}

	if b.conn == nil {
		return fmt.Errorf("unsubscribe failed as binding is not yet open")
	}

	if _, err := b.conn.Unsubscribe(ctx, u); err != nil {
		plog.Printf("unsubscribe %v failed: %v", topics, err)
		return services.RetryableErrorf("unsubscribe %v failed: %w", topics, err)
	}
	return nil
}

func (b *Mqtt5Binding) resubscribe() {
	if err := b.subscribe(b.getTopics()...); err != nil {
		plog.Printf("resubscribe failed: %v", err)
	}
}

func (b *Mqtt5Binding) getTopics() []string {
	eventTopics := b.eventRouter.GetTopics()
	actionTopics := b.actionRouter.GetTopics()
	actionResultTopics := b.actionResultRouter.GetTopics()
	queryTopics := b.queryRouter.GetTopics()
	queryResultTopics := b.queryResultRouter.GetTopics()
	topicsLen := len(eventTopics) + len(actionTopics) + len(actionResultTopics) + len(queryTopics) + len(queryResultTopics)
	topics := make([]string, topicsLen)
	i := 0
	i += copy(topics[i:], eventTopics)
	i += copy(topics[i:], actionTopics)
	i += copy(topics[i:], actionResultTopics)
	i += copy(topics[i:], queryTopics)
	i += copy(topics[i:], queryResultTopics)
	return topics
}

func (b *Mqtt5Binding) handle(p *paho.Publish) {
	if p.Properties == nil || p.Properties.User == nil {
		plog.Printf("handle: discard incoming topic %s without Properties", p.Topic)
		return
	}
	levels := strings.Split(p.Topic, "/")
	levelPattern, typeName := levels[2], levels[3]
	id, source := p.Properties.User.Get(userPropId), p.Properties.User.Get(userPropSource)
	routeFilter := mqttRouteFilter{Topic: p.Topic}
	correlationId := p.Properties.CorrelationData
	responseTopic := p.Properties.ResponseTopic
	if _, err := api.ToScope(levels[1]); err != nil {
		plog.Printf("handle: error on incoming topic: %v", err)
		return
	}

	switch levelPattern {
	case levelEvent:
		event := api.Event{
			Type:   typeName,
			Id:     id,
			Source: source,
			Time:   p.Properties.User.Get(userPropTime),
			Data:   p.Payload,
			DataContentType: p.Properties.User.Get(userPropDataContentType),
		}
		b.eventRouter.Dispatch(routeFilter, event)
	case levelAction:
		actionCb := api.ActionWithCallback{
			Action: api.Action{
				Type:   typeName,
				Id:     id,
				Source: source,
				Params: p.Payload,
				DataContentType: p.Properties.User.Get(userPropDataContentType),
			},
			Callback: func(result api.ActionResult) error {
				return b.publishActionResult(result, responseTopic, correlationId)
			},
		}
		b.actionRouter.Dispatch(routeFilter, actionCb)
	case levelActionResult:
		seqNo, err := strconv.ParseInt(p.Properties.User.Get(userPropSequenceNumber), 10, 64)
		if err != nil {
			plog.Printf("handle: error on SequenceNumber: %v", err)
			return
		}
		result := api.ActionResult{
			Context:        p.Properties.User.Get(userPropContext),
			Data:           p.Payload,
			DataContentType: p.Properties.User.Get(userPropDataContentType),
			SequenceNumber: seqNo,
		}
		routeFilter.CorrelationId = string(p.Properties.CorrelationData)
		b.actionResultRouter.Dispatch(routeFilter, result)
	case levelQuery:
		queryCb := api.QueryWithCallback{
			Query: api.Query{
				Type:            typeName,
				Id:              id,
				Source:          source,
				Data:            p.Payload,
				DataContentType: p.Properties.User.Get(userPropDataContentType),
			},
			Callback: func(result api.QueryResult) error {
				return b.publishQueryResult(result, responseTopic, correlationId)
			},
		}
		b.queryRouter.Dispatch(routeFilter, queryCb)
	case levelQueryResult:
		seqNo, err := strconv.ParseInt(p.Properties.User.Get(userPropSequenceNumber), 10, 64)
		if err != nil {
			plog.Printf("handle: error on SequenceNumber: %v", err)
			return
		}
		result := api.QueryResult{
			Context:         p.Properties.User.Get(userPropContext),
			Data:            p.Payload,
			DataContentType: p.Properties.User.Get(userPropDataContentType),
			SequenceNumber:  seqNo,
		}
		routeFilter.CorrelationId = string(p.Properties.CorrelationData)
		b.queryResultRouter.Dispatch(routeFilter, result)
	default:
		plog.Printf("handle: discard malformed incoming topic %s", p.Topic)
	}
}
