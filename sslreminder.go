/*
sslreminder is an application to check expiration dates of ssl certificates
and reminds expirations.

It can be configured via environmental variables.

Followings are mandatory.

	* HOSTS for comma separated hosts to be checked.
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
	"bytes"
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
	hosts         []string
	emails        []string
	thresholdDays int
	from          string
}

type sendgridConfig struct {
	username string
	password string
}

// Get expiration date for given host.
func GetExpiration(host string) (expiration time.Time, err error) {
	conn, err := tls.Dial("tcp", host+":443", &tls.Config{})
	if err != nil {
		log.Printf("ERROR dialing %v", host)
		return
	}
	defer conn.Close()
	state := conn.ConnectionState()
	certs := state.PeerCertificates

	if len(certs) == 0 {
		err = fmt.Errorf("No PeerCertificates found for %v", host)
		return
	}

	if certs[0] == nil {
		err = fmt.Errorf("First PeerCertificates is nil for %v", host)
		return
	}

	expiration = certs[0].NotAfter
	return
}

// Read an environmental variable.
// Exit process if it's empty or not set.
func envMandatory(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		log.Fatalf("%v must be set.", key)
	}
	return value
}

// Read an environmental variable.
// Returns defualtValue if it's empty or not set.
func envOptional(key string, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

// Read SendGrid related configs.
func readSendgridConfig() *sendgridConfig {
	return &sendgridConfig{
		envMandatory("SENDGRID_USERNAME"),
		envMandatory("SENDGRID_PASSWORD"),
	}
}

// Read general config.
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
		strings.Split(envMandatory("HOSTS"), ","),
		emails,
		int(threshold),
		envOptional("FROM", emails[0]),
	}
}

// Get a map from hosts to expiration dates.
func GetExpirationMap(config *config) map[string]time.Time {
	expirationMap := make(map[string]time.Time, len(config.hosts))

	for _, host := range config.hosts {
		exp, err := GetExpiration(host)
		if err != nil {
			log.Printf(
				"ERROR getting expiration time of %v: %v",
				host, err)
			continue
		}
		log.Printf("Expiration of %v is %v", host, exp)
		expirationMap[host] = exp
	}

	return expirationMap
}

// Check ssl certificates for given hosts, then remind if necessary.
func check(config *config, sgConfig *sendgridConfig, now time.Time) {
	log.Println("Check started")
	exMap := GetExpirationMap(config)
	threshold := now.AddDate(0, 0, config.thresholdDays)

	shouldRemind := false
	for _, ex := range exMap {
		if ex.Before(threshold) {
			shouldRemind = true
		}
	}

	if shouldRemind {
		remind(config, sgConfig, now, exMap)
	}
	log.Println("Check finished")
}

// A body of remind mail
func mailBody(config *config, now time.Time, exMap map[string]time.Time) string {
	threshold := now.AddDate(0, 0, config.thresholdDays)
	soon := make(map[string]time.Time)
	others := make(map[string]time.Time)
	for host, ex := range exMap {
		if ex.Before(threshold) {
			soon[host] = ex
			log.Printf("%v will be expired soon.", host)
		} else {
			others[host] = ex
		}
	}

	var buf bytes.Buffer
	buf.WriteString("Certificates of following hosts expires soon:\n")

	for host, ex := range soon {
		buf.WriteString(fmt.Sprintf("%v: %v\n", host, ex))
	}

	if len(others) > 0 {
		buf.WriteString("\nOthers have enough time to be expired:\n")
		for host, ex := range others {
			buf.WriteString(fmt.Sprintf("%v: %v\n", host, ex))
		}
	}
	return buf.String()
}

// Remind via email.
func remind(config *config, sgConfig *sendgridConfig, now time.Time,
	exMap map[string]time.Time) {
	sg := sendgrid.NewSendGridClient(sgConfig.username, sgConfig.password)
	msg := sendgrid.NewMail()
	msg.AddTos(config.emails)
	msg.SetSubject("REMINDER SSL certificate expiration")
	msg.SetText(mailBody(config, now, exMap))
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
	go check(config, sgConfig, time.Now())
	for {
		time.Sleep(24 * time.Hour)
		go check(config, sgConfig, time.Now())
	}
}
