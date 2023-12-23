package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/google/uuid"
	rabbitmq "github.com/wagslane/go-rabbitmq"
)

var publisher *rabbitmq.Publisher

func publishHandler() {
	pubch := make(chan os.Signal, 1)
	signal.Notify(pubch, syscall.SIGUSR1, syscall.SIGUSR2)

	//create test message to send
	ptest := KzMessage{
		EventCategory: "notification",
		EventName:     "push_req",
		AccountId:     "12a0f57aec7a40a1ccf6d959521d5682",
		AppName:       appname,
		AppVersion:    version,
		CallId:        uuid.New().String(),
		MsgId:         uuid.New().String(),
		Node:          myHostname,
		ServerId:      appname + "@" + myHostname,
		Description:   "Test message from RuhNet RabbitHunter",
		To:            "1234",
	}

	msg1, err := json.Marshal(ptest)
	if err != nil {
		logit(3, "Unable to marshal ptest into JSON! Published test message will be blank.")
	}

	msg2 := []byte(`{"test":true, "message":"test from ` + appnameFull + `","node":"` + myHostname + `"}`) //default second message

	for {
		select {
		case sig := <-pubch:
			logit(5, "Received signal "+fmt.Sprintf("%s", sig)+" on pubch channel. Publishing message... ")
			var msg []byte
			switch sig {
			case syscall.SIGUSR1:
				msg = msg1
			case syscall.SIGUSR2:
				msg = msg2
				msgFromFile, err := readPubMessageFile(appconf.PubMessageFile)
				if err == nil {
					msg = msgFromFile
				}
			}
			logit(7, "Publishing message to exchange '"+appconf.AmqpPubExch+"' with routing key '"+appconf.AmqpPubRoutingKey+"': \n"+string(msg))
			err := publisher.Publish(msg,
				[]string{appconf.AmqpPubRoutingKey},
				rabbitmq.WithPublishOptionsContentType("application/json"),
				rabbitmq.WithPublishOptionsExchange(appconf.AmqpPubExch),
				//rabbitmq.WithPublishOptionsExchange("amq.fanout"),
			)
			if err != nil {
				logit(3, "Error publishing message: "+err.Error())
				fmt.Println("Error publishing message: " + err.Error())
			} else {
				fmt.Println("PUBLISHED: \n" + string(msg) + "\n")
			}
		case <-done:
			fmt.Println("Shutting myself down...")
			return
		case <-time.After(100 * time.Millisecond):
			//case <-time.After(time.Second):
			//fmt.Println("tick")
		}
	}

}

func readPubMessageFile(filePath string) (msg []byte, err error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		logit(4, "Could not open config file: "+filePath+"\n"+err.Error())
		return msg, err
	}
	defer jsonFile.Close()
	fileBytes, _ := ioutil.ReadAll(jsonFile)

	//strip out // comments from file:
	re := regexp.MustCompile(`([\s]//.*)|(^//.*)`)
	fileCleanedBytes := re.ReplaceAll(fileBytes, nil)

	return fileCleanedBytes, err
}
