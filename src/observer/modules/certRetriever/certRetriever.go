package certRetriever

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"

	"observer"
)

type CertChain struct {
	Domain string   `json:"domain"`
	IP     string   `json:"ip"`
	Certs  []string `json:"certs"`
}

func init() {
	mig.RegisterModule("certretriever", func() interface{} {
		return new(Runner)
	})
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

func (r Runner) Run(msg []byte) string {
	certs, ip, err := checkHost(string(msg), "443", true)
	panicIf(err)
	if certs == nil {
		log.Println("no certificate retrieved from", string(msg))
		return
	}

	var chain = CertChain{}

	chain.Domain = string(msg)

	chain.IP = ip

	for _, cert := range certs {

		chain.Certs = append(chain.Certs, base64.StdEncoding.EncodeToString(cert.Raw))

	}

	jsonChain, er := json.MarshalIndent(chain, "", "    ")
	panicIf(er)

	return jsonChain
}
