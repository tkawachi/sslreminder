/*
sslreminder is an application to check expiration dates of ssl certificates
and reminds expirations.

It can be configured via environmental variables.

Followings are mandatory.

* DOMAINS for comma separated domains to be checked.
* EMAILS for comma separated email addresses.
* SENDGRID_USERNAME for SendGrid user name.
* SENDGRID_PASSWORD for SendGrid password.

Followings are optional.

* THRESHOLD_DAYS for threshold remaining days to remind. (default 30)
* FROM for from address. (default the first address in EMAILS)

It checks expiration dates once a day. It sends reminder via email
if any of certificates expire within THRESHOLD_DAYS.

*/
package main

import (
	"crypto/tls"
	"fmt"
	"github.com/sendgrid/sendgrid-go"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	domains       []string
	emails        []string
	thresholdDays int
	from          string
}

type sendgridConfig struct {
	username string
	password string
}

func GetExpiration(domain string) (expiration time.Time, err error) {
	conn, err := tls.Dial("tcp", domain+":443", &tls.Config{})
	if err != nil {
		log.Printf("ERROR dialing %v", domain)
		return
	}
	defer conn.Close()
	state := conn.ConnectionState()
	certs := state.PeerCertificates

	if len(certs) == 0 {
		err = fmt.Errorf("No PeerCertificates found for %v", domain)
		return
	}

	if certs[0] == nil {
		err = fmt.Errorf("First PeerCertificates is nil for %v", domain)
		return
	}

	expiration = certs[0].NotAfter
	return
}

func envMandatory(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		log.Fatalf("%v must be set.", key)
	}
	return value
}

func envOptional(key string, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func readSendgridConfig() *sendgridConfig {
	return &sendgridConfig{
		envMandatory("SENDGRID_USERNAME"),
		envMandatory("SENDGRID_PASSWORD"),
	}
}

func readConfig() *config {
	DEFAULT_THRESHOLD_DAYS := "30"
	thresholdString := envOptional("THRESHOLD_DAYS", DEFAULT_THRESHOLD_DAYS)
	threshold, err := strconv.ParseInt(thresholdString, 0, 0)
	if err != nil {
		log.Fatalf("Failed to parse THRESHOLD_DAYS: %v",
			thresholdString)
	}

	emails := strings.Split(envMandatory("EMAILS"), ",")

	return &config{
		strings.Split(envMandatory("DOMAINS"), ","),
		emails,
		int(threshold),
		envOptional("FROM", emails[0]),
	}
}

func GetExpirationMap(config *config) map[string]time.Time {
	expirationMap := make(map[string]time.Time, len(config.domains))

	for _, domain := range config.domains {
		exp, err := GetExpiration(domain)
		if err != nil {
			log.Printf(
				"ERROR getting expiration time of %v: %v",
				domain, err)
			continue
		}
		log.Printf("Expiration of %v is %v", domain, exp)
		expirationMap[domain] = exp
	}

	return expirationMap
}

func check(config *config, sgConfig *sendgridConfig, now time.Time) {
	log.Println("Start checking")
	exMap := GetExpirationMap(config)
	threshold := now.AddDate(0, 0, config.thresholdDays)

	shouldRemind := false
	for _, ex := range exMap {
		if ex.After(threshold) {
			shouldRemind = true
		}
	}

	if shouldRemind {
		remind(config, sgConfig, now, exMap)
	}
}

func remind(config *config, sgConfig *sendgridConfig, now time.Time,
	exMap map[string]time.Time) {
	sg := sendgrid.NewSendGridClient(sgConfig.username, sgConfig.password)
	msg := sendgrid.NewMail()
	msg.AddTos(config.emails)
	msg.SetSubject("SSL certificate expiration")
	// TODO generate body
	msg.SetText("")
	msg.SetFrom(config.from)
	err := sg.Send(msg)
	if err != nil {
		log.Printf("ERROR sending mail to %v: %v", config.emails, err)
	} else {
		log.Printf("Mail sent to %v", config.emails)
	}
}

func main() {
	config := readConfig()
	sgConfig := readSendgridConfig()
	check(config, sgConfig, time.Now())
}
