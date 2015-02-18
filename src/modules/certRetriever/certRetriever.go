package certRetriever

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"modules"
)

type CertChain struct {
	Domain string   `json:"domain"`
	IP     string   `json:"ip"`
	Certs  []string `json:"certs"`
}

func init() {
	modules.RegisterModule("certretriever", modules.ModulerInfo{InputQueue: "scan_ready_queue", Runner: new(Runner)})
}

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

type Runner struct {
}

//Does the main work by checking host connectivity and retrieving certificates ( if any )
func checkHost(domainName, port string, skipVerify bool) ([]*x509.Certificate, string, error) {

	config := tls.Config{InsecureSkipVerify: skipVerify}

	canonicalName := domainName + ":" + port

	ip := ""

	conn, err := tls.Dial("tcp", canonicalName, &config)

	if err != nil {
		return nil, ip, err
	}
	defer conn.Close()

	ip = strings.TrimRight(conn.RemoteAddr().String(), ":443")

	certs := conn.ConnectionState().PeerCertificates

	if certs == nil {
		return nil, ip, errors.New("Could not get server's certificate from the TLS connection.")
	}

	return certs, ip, nil
}

func (*Runner) Run(msg []byte, ch chan modules.ModuleResult) {

	log.Println("Start Retriever")

	res := modules.ModuleResult{
		Success:   false,
		Result:    nil,
		OutStream: "scan_results_queue",
		Errors:    nil,
	}

	log.Println("checking : ", string(msg))
	certs, ip, err := checkHost(string(msg), "443", true)
	panicIf(err)
	if certs == nil {
		res.Errors = append(res.Errors, fmt.Sprintf("no certificate retrieved from", string(msg)))
		ch <- res
	}

	var chain = CertChain{}

	chain.Domain = string(msg)

	chain.IP = ip

	for _, cert := range certs {

		chain.Certs = append(chain.Certs, base64.StdEncoding.EncodeToString(cert.Raw))

	}

	res.Result, err = json.MarshalIndent(chain, "", "    ")

	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}

	if len(res.Errors) == 0 {
		res.Success = true
	}

	ch <- res

	log.Println("End Retriever")
}
