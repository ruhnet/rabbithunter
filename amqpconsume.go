package main

import (
	"encoding/json"
	"fmt"

	rabbitmq "github.com/wagslane/go-rabbitmq"
)

var msgConsumer *rabbitmq.Consumer

type KzMessage struct {
	AccountId     string                 `json:"Account-ID"`
	AppName       string                 `json:"App-Name"`
	AppVersion    string                 `json:"App-Version"`
	CallId        string                 `json:"Call-ID"`
	EventCategory string                 `json:"Event-Category"`
	EventName     string                 `json:"Event-Name"`
	MsgId         string                 `json:"Msg-ID"`
	Node          string                 `json:"Node"`
	ServerId      string                 `json:"Server-ID"`
	Description   string                 `json:"Msg-Description"`
	To            string                 `json:"To"`
	Payload       map[string]interface{} `json:"Payload"`
}

func handleAmqpMsg(d rabbitmq.Delivery) rabbitmq.Action {
	// rabbitmq.Ack, rabbitmq.NackDiscard, rabbitmq.NackRequeue
	logit(7, "AMQP message received: "+string(d.Body))

	if appconf.FilterEvtCat == "*" && appconf.FilterEvtName == "*" && appconf.FilterEvtAppName == "*" {
		fmt.Println(string(d.Body))
		return rabbitmq.Ack
	}

	var msg KzMessage

	err := json.Unmarshal(d.Body, &msg)
	if err != nil {
		logit(5, "handleAmqpMsg(): Error unmarshalling AMQP message into map[string]interface{}...discarding. Message body: "+string(d.Body)+"\nUnmarshalling error: "+err.Error())
		return rabbitmq.NackDiscard
	}

	for _, appname := range msgAppNameFilters {
		if appconf.FilterEvtAppName == "*" || appname == msg.AppName { //only print messges that match a filter, or any if the filter is "*"
			for _, cat := range msgCatFilters {
				if appconf.FilterEvtCat == "*" || cat == msg.EventCategory {
					for _, name := range msgNameFilters {
						if appconf.FilterEvtName == "*" || name == msg.EventName {
							if appconf.LogLevel > 4 {
								fmt.Println("RoutingKey: ", string(d.RoutingKey))
							}
							fmt.Println(string(d.Body))
						}
					}
				}
			}
		}
	}

	return rabbitmq.Ack
}
