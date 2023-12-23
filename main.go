package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	rabbitmq "github.com/wagslane/go-rabbitmq"
)

const appnameFull string = "RuhNet RabbitHunter AMQP Utility"
const appname string = "rabbithunter"
const version string = "0.3"
const serverVersion string = "RuhNet " + appname + " v" + version
const website string = "https://ruhnet.co"

var startupTime time.Time
var myHostname, myHostnameShort string

const banner = `
   ____         __    _    _       __
  / ___\ __  __/ /_  / \  / /__ __/ /_
 / /_/ // /_/ / _  \/ / \/ //__\_  __/
/_/  \_\\ ___/_/ /_/_/ \__/ \__,/_/   %s
_____________________________________________________
`

var done = make(chan bool, 1)

var msgCatFilters []string
var msgNameFilters []string
var msgAppNameFilters []string

type AppConfig struct {
	AmqpURI           string `json:"amqp_uri" env:"AMQP_URI" default:"amqp://guest:guest@localhost:5672"`
	AmqpSubExch       string `json:"amqp_sub_exchange" env:"AMQP_SUB_EXCH" default:"callevt"`
	AmqpPubExch       string `json:"amqp_pub_exchange" env:"AMQP_PUB_EXCH" default:"pushes"`
	AmqpExchType      string `json:"amqp_exchange_type" env:"AMQP_EXCH_TYPE" default:"topic"`
	AmqpSubRoutingKey string `json:"amqp_sub_routing_key" env:"AMQP_SUB_ROUTING_KEY" default:"call.*.*"`
	AmqpPubRoutingKey string `json:"amqp_pub_routing_key" env:"AMQP_PUB_ROUTING_KEY" default:"notification.push.customapp.test"`
	AmqpWorkers       int    `json:"amqp_workers" env:"AMQP_WORKERS" default:"2"`
	LogFile           string `json:"log_file" env:"LOG_FILE" default:"/tmp/rabbithunter.log"`
	LogLevel          int    `json:"log_level" env:"LOG_LEVEL" default:"5"`
	PubMessageFile    string `json:"pub_message_file" env:"PUB_MSG_FILE" default:"./message.json"`
	FilterEvtCat      string `json:"filter_event_category" env:"FLT_EVT_CAT" default:"*"`    //allow comma separated string for multiple
	FilterEvtName     string `json:"filter_event_name" env:"FLT_EVT_NAME" default:"*"`       //allow comma separated string for multiple
	FilterEvtAppName  string `json:"filter_event_appname" env:"FLT_EVT_APPNAME" default:"*"` //allow comma separated string for multiple
}

func main() {
	var err error
	startupTime = time.Now()

	/////////////////////////////////////////////
	// Setup
	initConfig()

	fmt.Println("Configuration OK, starting " + appname + "...")
	logit(5, "Configuration OK, starting "+appname+"...")

	/////////////////////////////////////////////
	// Logging
	appLogFile, err := os.OpenFile(appconf.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		log.Println("Could not open log file: " + appconf.LogFile + "\n" + err.Error())
		appLogFile, err = os.OpenFile("/tmp/"+appname+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
		if err != nil {
			log.Fatal("Can't open even /tmp log file!\n" + err.Error())
		}
	}
	defer appLogFile.Close()
	//set other logging to same file
	log.SetOutput(appLogFile)

	//Startup Banner
	fmt.Printf(banner, website)
	fmt.Println(serverVersion + "\n")

	logit(5, appname+"v"+version+"  starting up... ")
	logit(5, fmt.Sprintf("Logging with level: %d", appconf.LogLevel))

	//Hostname
	myHostname, err = os.Hostname()
	logit(5, "Detecting my hostname... "+myHostname)
	if err != nil {
		log.Fatal("Hostname could not be auto-detected from system: " + err.Error())
	}
	myHostnameShort = strings.SplitN(myHostname, ".", 2)[0]

	//fmt.Println()

	/////////////////////////////////////////////
	// RabbitMQ Setup Connection
	fmt.Println("Connecting to RabbitMQ: " + appconf.AmqpURI)
	logit(6, "Connecting to RabbitMQ: "+appconf.AmqpURI)
	amqpConn, err := rabbitmq.NewConn(
		appconf.AmqpURI,
		rabbitmq.WithConnectionOptionsLogging,
	)
	if err != nil {
		fmt.Println("Unable to initialize RabbitMQ connection: " + err.Error())
		log.Fatal("Unable to initialize RabbitMQ connection: " + err.Error())
	}
	defer amqpConn.Close()

	/////////////////////////////////////////////
	// RabbitMQ Setup Consumer
	msgConsumer, err = rabbitmq.NewConsumer(
		amqpConn,
		handleAmqpMsg, //this is the function that we want to call to consume presence messages
		"q_rabbithunter_"+myHostnameShort,
		rabbitmq.WithConsumerOptionsExchangeName(appconf.AmqpSubExch),
		rabbitmq.WithConsumerOptionsExchangeKind(appconf.AmqpExchType),
		rabbitmq.WithConsumerOptionsRoutingKey(appconf.AmqpSubRoutingKey),
		rabbitmq.WithConsumerOptionsConsumerName("consumer_rabbithunter_"+myHostnameShort),
		rabbitmq.WithConsumerOptionsQueueAutoDelete,
		rabbitmq.WithConsumerOptionsConcurrency(appconf.AmqpWorkers),
		//rabbitmq.WithConsumerOptionsQuorum,
		//rabbitmq.WithConsumerOptionsQueueDurable,
		//rabbitmq.WithConsumerOptionsExchangeDeclare,
		//rabbitmq.WithConsumerOptionsBindingExchangeDurable,
	)

	if err != nil {
		fmt.Println("Unable to initialize RabbitMQ consumer: " + err.Error())
		log.Fatal("Unable to initialize RabbitMQ consumer: " + err.Error())
	}
	defer msgConsumer.Close()

	logit(6, "Consuming on '"+appconf.AmqpExchType+"' exchange: '"+appconf.AmqpSubExch+"' with routing key: '"+appconf.AmqpSubRoutingKey+"' using queue: 'consumer_rabbithunter_"+myHostnameShort+"'.")

	/////////////////////////////////////////////
	// RabbitMQ Setup Publisher
	publisher, err = rabbitmq.NewPublisher(
		amqpConn,
		rabbitmq.WithPublisherOptionsLogging,
		rabbitmq.WithPublisherOptionsExchangeKind(appconf.AmqpExchType),
		rabbitmq.WithPublisherOptionsExchangeName(appconf.AmqpPubExch),
		//rabbitmq.WithPublisherOptionsExchangeDeclare,
	)
	if err != nil {
		fmt.Println("Unable to initialize RabbitMQ publisher: " + err.Error())
		log.Fatal("Unable to initialize RabbitMQ publisher: " + err.Error())
	}
	defer publisher.Close()

	logit(7, "Publisher configured on '"+appconf.AmqpExchType+"' exchange: '"+appconf.AmqpPubExch+"' with routing key: '"+appconf.AmqpPubRoutingKey+"'.")

	publisher.NotifyReturn(func(r rabbitmq.Return) {
		logit(4, fmt.Sprintf("RabbitMQ published message returned from server: %s\n", string(r.Body)))
	})
	publisher.NotifyPublish(func(c rabbitmq.Confirmation) {
		logit(7, fmt.Sprintf("Message confirmed from RabbitMQ server. tag: %v, ack: %v\n", c.DeliveryTag, c.Ack))
	})

	/////////////////////////////////////////////
	// Message Filters
	msgCatFilters = strings.Split(appconf.FilterEvtCat, ",")
	msgNameFilters = strings.Split(appconf.FilterEvtName, ",")
	msgAppNameFilters = strings.Split(appconf.FilterEvtAppName, ",")
	fmt.Println("Filtering event app names matching:", msgAppNameFilters)
	fmt.Println("Filtering event categories matching:", msgCatFilters)
	fmt.Println("Filtering event names matching:", msgNameFilters)

	/////////////////////////////////////////////
	logit(6, "Using "+fmt.Sprintf("%d", appconf.AmqpWorkers)+" concurrent workers to process AMQP messages.")

	log.Println(appnameFull + " system started.\n-----> [READY]")
	fmt.Println(appnameFull + " system started.\n-----> [READY]")

	// block main thread - wait for shutdown signal
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Printf("Received signal: ")
		fmt.Println(sig)
		done <- true
	}()

	publishHandler() //this func will block the main thread, until receiving on the 'done' channel.
}

func logit(level int, message string) {
	if appconf.LogLevel >= int(level) {
		log.Println(message)
	}
	if appconf.LogLevel >= 8 {
		fmt.Println(message)
	}
}
