// Copyright (c) 2014 The SurgeMQ Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"

	glog "github.com/Sirupsen/logrus"
	"github.com/surgemq/message"
	"github.com/surgemq/surgemq/sessions"
)

var (
	errDisconnect  = errors.New("Disconnect")
	errRouterError = errors.New("Router")
)

// processor() reads messages from the incoming buffer and processes them
func (this *service) processor() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			//glog.Errorf("(%s) Recovering from panic: %v", this.cid(), r)
		}

		this.wgStopped.Done()
		this.stop()

		glog.Infof("(%s) Stopping processor", this.cid())
	}()

	glog.Infof("(%s) Starting processor", this.cid())

	this.wgStarted.Done()

	for {
		// 1. Find out what message is next and the size of the message
		mtype, total, err := this.peekMessageSize()
		if err != nil {
			//if err != io.EOF {
			glog.Errorf("(%s) Error peeking next message size: %v", this.cid(), err)
			//}
			return
		}
		msg, n, err := this.peekMessage(mtype, total)
		if err != nil {
			//if err != io.EOF {
			glog.Errorf("(%s) Error peeking next message: %v", this.cid(), err)
			//}
			return
		}

		//glog.Debugf("(%s) Received: %s", this.cid(), msg)
		this.inStat.increment(int64(n))

		// 5. Process the read message
		err = this.processIncoming(msg)
		if err != nil {
			if err != errDisconnect && err != errRouterError {
				glog.Errorf("(%s) Error processing %s: %v", this.cid(), msg.Name(), err)
			} else {
				return
			}
		}

		// 7. We should commit the bytes in the buffer so we can move on
		_, err = this.in.ReadCommit(total)
		if err != nil {
			if err != io.EOF {
				glog.Errorf("(%s) Error committing %d read bytes: %v", this.cid(), total, err)
			}
			return
		}

		// 7. Check to see if done is closed, if so, exit
		if this.isDone() && this.in.Len() == 0 {
			return
		}
		//if this.inStat.msgs%1000 == 0 {
		//glog.Debugf("(%s) Going to process message %d", this.cid(), this.inStat.msgs)
		//}
	}
}

func (this *service) processIncoming(msg message.Message) error {
	var err error = nil

	switch msg := msg.(type) {
	case *message.PublishMessage:
		// For PUBLISH message, we should figure out what QoS it is and process accordingly
		// If QoS == 0, we should just take the next step, no ack required
		// If QoS == 1, we should send back PUBACK, then take the next step
		// If QoS == 2, we need to put it in the ack queue, send back PUBREC
		err = this.processPublish(msg)

	case *message.PubackMessage:
		// For PUBACK message, it means QoS 1, we should send to ack queue
		this.sess.Pub1ack.Ack(msg)
		this.processAcked(this.sess.Pub1ack)

	case *message.PubrecMessage:
		// For PUBREC message, it means QoS 2, we should send to ack queue, and send back PUBREL
		if err = this.sess.Pub2out.Ack(msg); err != nil {
			break
		}

		resp := message.NewPubrelMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = this.writeMessage(resp)

	case *message.PubrelMessage:
		// For PUBREL message, it means QoS 2, we should send to ack queue, and send back PUBCOMP
		if err = this.sess.Pub2in.Ack(msg); err != nil {
			break
		}

		this.processAcked(this.sess.Pub2in)

		resp := message.NewPubcompMessage()
		resp.SetPacketId(msg.PacketId())
		_, err = this.writeMessage(resp)

	case *message.PubcompMessage:
		// For PUBCOMP message, it means QoS 2, we should send to ack queue
		if err = this.sess.Pub2out.Ack(msg); err != nil {
			break
		}

		this.processAcked(this.sess.Pub2out)

	case *message.SubscribeMessage:
		// For SUBSCRIBE message, we should add subscriber, then send back SUBACK
		//如果client不是集群的client
		if this.route == nil && this.server.routeInfo.ID != "" {
			this.server.mu.Lock()
			for _, route := range this.server.route_client {
				route.writeMessage(msg)
			}
			this.server.mu.Unlock()
		}
		if this.route != nil {
			glog.Info("Receive router subcribe for ", msg.String())
			return this.processRouterSubscribe(msg)
		} else {
			return this.processSubscribe(msg)
		}
	case *message.SubackMessage:
		// For SUBACK message, we should send to ack queue
		this.sess.Suback.Ack(msg)
		this.processAcked(this.sess.Suback)

	case *message.UnsubscribeMessage:
		// For UNSUBSCRIBE message, we should remove subscriber, then send back UNSUBACK

		if this.route != nil {
			return this.processRouterUnsubscribe(msg)
		} else {
			this.server.mu.Lock()
			for _, route := range this.server.route_client {
				route.writeMessage(msg)
			}
			this.server.mu.Unlock()

			return this.processUnsubscribe(msg)
		}
	case *message.UnsubackMessage:
		// For UNSUBACK message, we should send to ack queue
		this.sess.Unsuback.Ack(msg)
		this.processAcked(this.sess.Unsuback)

	case *message.PingreqMessage:
		// For PINGREQ message, we should send back PINGRESP
		resp := message.NewPingrespMessage()
		_, err = this.writeMessage(resp)

	case *message.PingrespMessage:
		if this.route == nil {
			this.sess.Pingack.Ack(msg)
			this.processAcked(this.sess.Pingack)
		}
	case *message.DisconnectMessage:
		// For DISCONNECT message, we should quit
		this.sess.Cmsg.SetWillFlag(false)
		return errDisconnect
	case *message.RouteMessage:
		err = this.processRouteInfo(msg)
	default:
		return fmt.Errorf("(%s) invalid message type %s.", this.cid(), msg.Name())
	}

	if err != nil {
		glog.Debugf("(%s) Error processing acked message: %v", this.cid(), err)
	}
	return err
}

func (this *service) processRouteInfo(msg *message.RouteMessage) error {
	this.mu.Lock()
	if this.route == nil || this.conn == nil {
		this.mu.Unlock()
		return errRouterError
	}

	s := this.server
	remoteID := this.route.remoteID
	//解析info
	var info Info
	err := json.Unmarshal(msg.GetInfo(), &info)
	if err != nil {
		glog.Errorf("(%s) Error processing router info : %v", this.cid(), err)
		return errRouterError
	}
	if remoteID != "" && remoteID != info.ID {
		this.mu.Unlock()
		s.processImplicitRoute(info)
		return nil
	}

	this.route.remoteID = info.ID

	// Detect route to self.
	if this.route.remoteID == this.server.routeInfo.ID {
		this.mu.Unlock()
		return errRouterError
	}

	// If we do not know this route's URL, construct one on the fly
	// from the information provided.
	if this.route.url == nil {
		// Add in the URL from host and port
		url, err := url.Parse(info.IP)
		if err != nil {
			glog.Errorf("Error parsing URL from INFO: %v\n", err)
			this.mu.Unlock()
			return errRouterError
		}
		this.route.url = url
	}

	// Check to see if we have this remote already registered.
	// This can happen when both servers have routes to each other.
	this.mu.Unlock()

	if added, sendInfo := s.addRoute(this); added {
		glog.Debugf("Registering remote route %q", info.ID)
		// Send our local subscriptions to this route.
		s.sendLocalSubsToRoute(this)
		if sendInfo {
			// Need to get the remote IP address.
			this.mu.Lock()
			info.IP = fmt.Sprintf("%s", this.route.url)
			this.mu.Unlock()
			// Now let the known servers know about this new route
			s.forwardNewRouteInfoToKnownServers(info)
		}
		return nil
	} else {
		glog.Debugf("Detected duplicate remote route %q", info.ID)
		return errRouterError
	}
}

func (this *service) processAcked(ackq *sessions.Ackqueue) {
	for _, ackmsg := range ackq.Acked() {
		// Let's get the messages from the saved message byte slices.
		msg, err := ackmsg.Mtype.New()
		if err != nil {
			glog.Errorf("process/processAcked: Unable to creating new %s message: %v", ackmsg.Mtype, err)
			continue
		}

		if _, err := msg.Decode(ackmsg.Msgbuf); err != nil {
			glog.Errorf("process/processAcked: Unable to decode %s message: %v", ackmsg.Mtype, err)
			continue
		}

		ack, err := ackmsg.State.New()
		if err != nil {
			glog.Errorf("process/processAcked: Unable to creating new %s message: %v", ackmsg.State, err)
			continue
		}

		if _, err := ack.Decode(ackmsg.Ackbuf); err != nil {
			glog.Errorf("process/processAcked: Unable to decode %s message: %v", ackmsg.State, err)
			continue
		}

		//glog.Debugf("(%s) Processing acked message: %v", this.cid(), ack)

		// - PUBACK if it's QoS 1 message. This is on the client side.
		// - PUBREL if it's QoS 2 message. This is on the server side.
		// - PUBCOMP if it's QoS 2 message. This is on the client side.
		// - SUBACK if it's a subscribe message. This is on the client side.
		// - UNSUBACK if it's a unsubscribe message. This is on the client side.
		switch ackmsg.State {
		case message.PUBREL:
			// If ack is PUBREL, that means the QoS 2 message sent by a remote client is
			// releassed, so let's publish it to other subscribers.
			if err = this.onPublish(msg.(*message.PublishMessage)); err != nil {
				glog.Errorf("(%s) Error processing ack'ed %s message: %v", this.cid(), ackmsg.Mtype, err)
			}

		case message.PUBACK, message.PUBCOMP, message.SUBACK, message.UNSUBACK, message.PINGRESP:
			glog.Debugf("process/processAcked: %s", ack)
			// If ack is PUBACK, that means the QoS 1 message sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.

			// If ack is PUBCOMP, that means the QoS 2 message sent by this service got
			// ack'ed. There's nothing to do other than calling onComplete() below.

			// If ack is SUBACK, that means the SUBSCRIBE message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.

			// If ack is UNSUBACK, that means the SUBSCRIBE message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.

			// If ack is PINGRESP, that means the PINGREQ message sent by this service
			// got ack'ed. There's nothing to do other than calling onComplete() below.

			err = nil

		default:
			glog.Errorf("(%s) Invalid ack message type %s.", this.cid(), ackmsg.State)
			continue
		}

		// Call the registered onComplete function
		if ackmsg.OnComplete != nil {
			onComplete, ok := ackmsg.OnComplete.(OnCompleteFunc)
			if !ok {
				glog.Errorf("process/processAcked: Error type asserting onComplete function: %v", reflect.TypeOf(ackmsg.OnComplete))
			} else if onComplete != nil {
				if err := onComplete(msg, ack, nil); err != nil {
					glog.Errorf("process/processAcked: Error running onComplete(): %v", err)
				}
			}
		}
	}
}

// For PUBLISH message, we should figure out what QoS it is and process accordingly
// If QoS == 0, we should just take the next step, no ack required
// If QoS == 1, we should send back PUBACK, then take the next step
// If QoS == 2, we need to put it in the ack queue, send back PUBREC
func (this *service) processPublish(msg *message.PublishMessage) error {
	switch msg.QoS() {
	case message.QosExactlyOnce:
		this.sess.Pub2in.Wait(msg, nil)

		resp := message.NewPubrecMessage()
		resp.SetPacketId(msg.PacketId())

		_, err := this.writeMessage(resp)
		return err

	case message.QosAtLeastOnce:
		resp := message.NewPubackMessage()
		resp.SetPacketId(msg.PacketId())

		if _, err := this.writeMessage(resp); err != nil {
			return err
		}

		return this.onPublish(msg)

	case message.QosAtMostOnce:
		return this.onPublish(msg)
	}

	return fmt.Errorf("(%s) invalid message QoS %d.", this.cid(), msg.QoS())
}

// For SUBSCRIBE message, we should add subscriber, then send back SUBACK
func (this *service) processSubscribe(msg *message.SubscribeMessage) error {
	resp := message.NewSubackMessage()
	resp.SetPacketId(msg.PacketId())

	// Subscribe to the different topics
	var retcodes []byte

	topics := msg.Topics()
	qos := msg.Qos()

	this.rmsgs = this.rmsgs[0:0]

	for i, t := range topics {

		if this.route == nil {
			qos[i] = 0x00
		}

		rqos, err := this.topicsMgr.Subscribe(t, qos[i], &this.onpub)
		if err != nil {
			return err
		}
		this.sess.AddTopic(string(t), qos[i])

		retcodes = append(retcodes, rqos)

		// yeah I am not checking errors here. If there's an error we don't want the
		// subscription to stop, just let it go.
		this.topicsMgr.Retained(t, &this.rmsgs)
		glog.Infof("(%s) topic = %s, retained count = %d", this.cid(), string(t), len(this.rmsgs))
		topics, qoss, _ := this.sess.Topics()
		glog.Infof("topics = %s qoss = %s", topics, qoss)
	}

	if err := resp.AddReturnCodes(retcodes); err != nil {
		return err
	}

	if this.route == nil {
		if _, err := this.writeMessage(resp); err != nil {
			return err
		}
	}

	for _, rm := range this.rmsgs {
		if err := this.publish(rm, nil); err != nil {
			glog.Errorf("service/processSubscribe: Error publishing retained message: %v", err)
			return err
		}
	}

	return nil
}

func (this *service) processRouterSubscribe(msg *message.SubscribeMessage) error {

	topics := msg.Topics()

	qos := msg.Qos()

	for i, t := range topics {
		if this.route == nil {
			qos[i] = 0x00
		}

		_, err := this.topicsMgr.Subscribe(t, qos[i], &this.onpub)
		if err != nil {
			return err
		}
		this.topics[string(t)] = struct{}{}
	}

	return nil
}

func (this *service) processRouterUnsubscribe(msg *message.UnsubscribeMessage) error {
	topics := msg.Topics()

	for _, t := range topics {
		err := this.topicsMgr.Unsubscribe(t, &this.onpub)
		if err != nil {
			return err
		}
		delete(this.topics, string(t))
	}

	return nil
}

// For UNSUBSCRIBE message, we should remove the subscriber, and send back UNSUBACK
func (this *service) processUnsubscribe(msg *message.UnsubscribeMessage) error {
	topics := msg.Topics()

	for _, t := range topics {
		this.topicsMgr.Unsubscribe(t, &this.onpub)
		this.sess.RemoveTopic(string(t))
	}

	resp := message.NewUnsubackMessage()
	resp.SetPacketId(msg.PacketId())

	_, err := this.writeMessage(resp)
	tps, _, _ := this.sess.Topics()
	glog.Infof("topics = %s", tps)
	return err
}

// onPublish() is called when the server receives a PUBLISH message AND have completed
// the ack cycle. This method will get the list of subscribers based on the publish
// topic, and publishes the message to the list of subscribers.
func (this *service) onPublish(msg *message.PublishMessage) error {
	if msg.Retain() {
		if err := this.topicsMgr.Retain(msg); err != nil {
			glog.Errorf("(%s) Error retaining message: %v", this.cid(), err)
		}
	}

	err := this.topicsMgr.Subscribers(msg.Topic(), msg.QoS(), &this.subs, &this.qoss)
	if err != nil {
		glog.Errorf("(%s) Error retrieving subscribers list: %v", this.cid(), err)
		return err
	}

	msg.SetRetain(false)

	glog.Debugf("(%s  %d) Publishing to topic =  %v msg = %v ", this.cid(), len(this.subs), string(msg.Topic()), string(msg.Payload()))

	for _, s := range this.subs {
		if s != nil {
			fn, ok := s.(*OnPublishFunc)
			if !ok {
				glog.Errorf("Invalid onPublish Function")
				return fmt.Errorf("Invalid onPublish Function")
			} else {
				if this.route == nil {
					(*fn)(msg, false)
				} else {
					(*fn)(msg, true)
				}
			}
		}
	}

	return nil
}
