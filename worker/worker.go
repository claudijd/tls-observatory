package worker

import (
	"fmt"
	"os"

	"github.com/mozilla/tls-observatory/certificate"
	"github.com/mozilla/tls-observatory/connection"
	"github.com/mozilla/tls-observatory/database"
)

// Result contains all the info each worker can provide as a result,
// through the result channel, to the caller.
type Result struct {
	Success    bool     `json:"success"`
	WorkerName string   `json:"name"`
	Result     []byte   `json:"result"`
	Errors     []string `json:"errors"`
}

// Input holds all the info that is given as input to each scanner.
type Input struct {
	Certificate certificate.Certificate
	Connection  connection.Stored
	Scanid      int64
	DBHandle    *database.DB
}

// Info represents the information that every worker gives about itself at the
// time of registration.
// Runner is the "object" on which the run method is going to be called.
type Info struct {
	Runner      Worker
	Description string
}

// AvailableWorkers is the global variable that contains all the workers that have registered
// themselves as available.
var AvailableWorkers = make(map[string]Info)

// RegisterWorker is called by each worker in order to register itself as available.
func RegisterWorker(name string, info Info) {
	if _, exist := AvailableWorkers[name]; exist {
		fmt.Fprintf(os.Stderr, "RegisterWorker: a worker named %q has already been registered.\nAre you trying to import the same worker twice?\n", name)
		os.Exit(1)
	}
	AvailableWorkers[name] = info
}

// Worker is the interface that is used to provide transparent running of any type of worker
// from the main application.
type Worker interface {
	Run(Input, chan Result)
}

type HasAnalysisPrinter interface {
	AnalysisPrinter([]byte) ([]string, error)
}

type HasAssertor interface {
	Assertor([]byte, []byte) (bool, []byte, error)
}
