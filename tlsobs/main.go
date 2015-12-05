package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mozilla/tls-observatory/certificate"
	"github.com/mozilla/tls-observatory/connection"
	"github.com/mozilla/tls-observatory/database"
)

func usage() {
	fmt.Fprintf(os.Stderr, "%s - Scan a site using Mozilla's TLS Observatory\n"+
		"Usage: %s <options> mozilla.org\n",
		os.Args[0], os.Args[0])
}

type scan struct {
	ID string `json:"scan_id"`
}

func main() {
	var (
		err     error
		scan    scan
		results database.Scan
	)
	flag.Usage = func() {
		usage()
		flag.PrintDefaults()
	}
	var observatory = flag.String("observatory", "https://tls-observatory.services.mozilla.com", "URL of the observatory")
	flag.Parse()
	if len(flag.Args()) != 1 {
		fmt.Println("error: must take only 1 non-flag argument as the target")
		usage()
		os.Exit(1)
	}
	resp, err := http.Post(*observatory+"/api/v1/scan?target="+flag.Arg(0),
		"application/json", nil)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(body, &scan)
	if err != nil {
		panic(err)
	}
	has_cert := false
	for {
		resp, err = http.Get(*observatory + "/api/v1/results?id=" + scan.ID)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(body, &results)
		if err != nil {
			panic(err)
		}
		if results.Cert_id > 0 && !has_cert {
			printCert(results.Cert_id, *observatory)
			has_cert = true
		}
		if results.Complperc == 100 {
			break
		}
		if has_cert {
			fmt.Printf(".")
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Printf("\n")
	printConnection(results.Conn_info)
}

func printCert(id int64, observatory string) {
	var (
		cert certificate.Certificate
		san  string
	)
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/certificate?id=%d", observatory, id))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(body, &cert)
	if err != nil {
		panic(err)
	}
	if len(cert.X509v3Extensions.SubjectAlternativeName) == 0 {
		san = "- none"
	} else {
		for _, name := range cert.X509v3Extensions.SubjectAlternativeName {
			san += "- " + name + "\n"
		}
	}
	fmt.Printf(`
Subject  %s	
SubjectAlternativeName
%sIssuer   %s
Validity %s to %s
CA       %t
SHA1     %s
SHA256   %s
SigAlg   %s
%s`, cert.Subject.String(), san, cert.Issuer.String(),
		cert.Validity.NotBefore.Format(time.RFC3339), cert.Validity.NotAfter.Format(time.RFC3339),
		cert.CA, cert.Hashes.SHA1, cert.Hashes.SHA256, cert.SignatureAlgorithm, cert.Anomalies)
}

func printConnection(c connection.Stored) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 5, 0, 1, ' ', 0)
	fmt.Fprintf(w, "prio\tcipher\tprotocols\tpfs\tcurves\n")
	for i, entry := range c.CipherSuite {
		var (
			protos string
		)
		for _, proto := range entry.Protocols {
			if protos != "" {
				protos += ","
			}
			protos += proto
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1,
			entry.Cipher, protos, entry.PFS, strings.Join(entry.Curves, ","))
	}
	w.Flush()
	fmt.Printf(`OCSP Stapling        %t
Server Side Ordering %t
Curves Fallback      %t
`, c.CipherSuite[0].OCSPStapling, c.ServerSide, c.CurvesFallback)
}