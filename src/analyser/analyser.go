package main

import (
	// stdlib packages

	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	// custom packages
	"config"
	"modules"

	_ "modules/certAnalyser"

	// 3rd party dependencies
	// elastigo "github.com/mattbaird/elastigo/lib"
	"github.com/mattbaird/elastigo/lib"
	"github.com/streadway/amqp"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func panicIf(err error) bool {
	if err != nil {
		log.Println(fmt.Sprintf("%s", err))
		return true
	}

	return false
}

func printIntro() {
	fmt.Println(`
	##################################
	#         CertAnalyzer           #
	##################################
	`)
}

var wg sync.WaitGroup
var es *elastigo.Conn

func main() {
	var (
		err error
	)
	cores := runtime.NumCPU()
	runtime.GOMAXPROCS(cores * 2)

	printIntro()

	conf := config.AnalyzerConfig{}

	var cfgFile string
	flag.StringVar(&cfgFile, "c", "/etc/observer/analyzer.cfg", "Input file csv format")
	flag.Parse()

	_, err = os.Stat(cfgFile)
	failOnError(err, "Missing configuration file from '-c' or /etc/observer/retriever.cfg")

	conf, err = config.AnalyzerConfigLoad(cfgFile)
	if err != nil {
		conf = config.GetAnalyzerDefaults()
	}

	conn, err := amqp.Dial(conf.General.RabbitMQRelay)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	es = elastigo.NewConn()
	es.Domain = conf.General.ElasticSearch

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.Qos(
		3,     // prefetch count
		0,     // prefetch size
		false, // global
	)

	failOnError(err, "Failed to set QoS")

	for name, modInfo := range modules.AvailableModules {

		log.Println("Found module", name)

		q, err := ch.QueueDeclare(
			modInfo.InputQueue, // name
			true,               // durable
			false,              // delete when unused
			false,              // exclusive
			false,              // no-wait
			nil,                // arguments
		)

		if err != nil {
			log.Println("Could not declare queue ", modInfo.InputQueue, "with error - ", err.Error())
			continue
		}

		go func() {

			msgs, err := ch.Consume(
				q.Name, // queue
				"",     // consumer
				true,   // auto-ack
				false,  // exclusive
				false,  // no-local
				false,  // no-wait
				nil,    // args
			)

			if err != nil {
				log.Println("Could not register consumer for ", name, " with error:", err.Error())
			}

			resChan := make(chan modules.ModuleResult)

			go func() {

				for {
					m := <-resChan

					log.Println("got result for ", m.OutStream)

					if m.Success {

						err = ch.Publish(
							"",          // exchange
							m.OutStream, // routing key
							false,       // mandatory
							false,
							amqp.Publishing{
								DeliveryMode: amqp.Persistent,
								ContentType:  "text/plain",
								Body:         []byte(m.Result),
							})
						panicIf(err)
					} else {
						log.Println(m.Errors)
					}
				}
			}()

			log.Println("Waiting for messages on :", modInfo.InputQueue)

			for d := range msgs {

				log.Println("got message -- ", string(d.Body), " -- on :", modInfo.InputQueue)

				go modInfo.Runner.(modules.Moduler).Run(d.Body, resChan)
			}

		}()

	}
	select {}
}
