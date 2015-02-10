package main

import (
	// stdlib packages

	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	// custom packages
	"config"
	"modules"

	_ "modules/certRetriever"

	// 3rd party dependencies
	"github.com/streadway/amqp"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func panicIf(err error) {
	if err != nil {
		log.Println(fmt.Sprintf("%s", err))
	}
}

func releaseSemaphore() {
	sem <- true
}

func printIntro() {
	fmt.Println(`
	#################################
	#            Retriever          #
	#################################
	`)
}

var sem chan bool

func main() {
	var (
		err error
	)
	cores := runtime.NumCPU()
	runtime.GOMAXPROCS(cores * 2)

	printIntro()

	conf := config.RetrieverConfig{}

	var cfgFile string
	flag.StringVar(&cfgFile, "c", "/etc/observer/retriever.cfg", "Input file csv format")
	flag.Parse()

	_, err = os.Stat(cfgFile)
	failOnError(err, "Missing configuration file from '-c' or /etc/observer/retriever.cfg")

	conf, err = config.RetrieverConfigLoad(cfgFile)
	if err != nil {
		conf = config.GetRetrieverDefaults()
	}

	conn, err := amqp.Dial(conf.General.RabbitMQRelay)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

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
					}
				}
			}()

			for d := range msgs {
				go modInfo.RunnerFunc(d.Body, resChan)
			}

		}()

	}
	select {}
}
