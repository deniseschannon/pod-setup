package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
)

const (
	sysctlKey    = "SYSCTL"
	dnsAppendKey = "DNS_APPEND"
	dnsSearchKey = "DNS_SEARCH"

	resolvConfLocation = "/etc/resolv.conf"

	rancherNameserver = "169.254.169.250"
)

func main() {
	sysctlSettings := os.Getenv(sysctlKey)
	if sysctlSettings != "" {
		sysctlSetup(sysctlSettings)
	}
	dnsSearch := os.Getenv(dnsSearchKey)
	dnsAppend := os.Getenv(dnsAppendKey)
	if dnsAppend != "" && dnsSearch != "" {
		dnsSetup(dnsSearch, dnsAppend == "true")
	}
}

func sysctlSetup(sysctlSettings string) {
	for _, setting := range strings.Split(sysctlSettings, ",") {
		parts := strings.Split(setting, "=")
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		pathParts := []string{"/proc", "sys"}
		pathParts = append(pathParts, strings.Split(key, ".")...)
		path := path.Join(pathParts...)
		if err := ioutil.WriteFile(path, []byte(value), 0644); err != nil {
			logrus.Errorf("Failed to set sysctl key %s: %v", value, err)
		}
	}
}

func dnsSetup(dnsSearch string, dnsAppend bool) error {
	input, err := os.Open(resolvConfLocation)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	scanner := bufio.NewScanner(input)
	searchSet := false
	nameserverSet := false
	for scanner.Scan() {
		text := scanner.Text()

		if strings.Contains(text, rancherNameserver) {
			nameserverSet = true
		} else if strings.HasPrefix(text, "nameserver") {
			text = "# " + text
		}

		if strings.HasPrefix(text, "search") {
			domainsToBeAdded := []string{}
			for _, domain := range strings.Split(dnsSearch, ",") {
				if strings.Contains(text, " "+domain) {
					continue
				}
				domainsToBeAdded = append(domainsToBeAdded, domain)
			}

			if dnsAppend {
				text = text + " " + strings.Join(domainsToBeAdded, " ")
			} else {
				text = strings.Replace(text, "search", "search "+strings.Join(domainsToBeAdded, " "), 1)
			}

			searchSet = true
		}

		if _, err := buffer.Write([]byte(text)); err != nil {
			return err
		}

		if _, err := buffer.Write([]byte("\n")); err != nil {
			return err
		}
	}

	if !searchSet {
		buffer.Write([]byte("search " + strings.ToLower(strings.Join(strings.Split(dnsSearch, ","), " "))))
		buffer.Write([]byte("\n"))
	}

	if !nameserverSet {
		buffer.Write([]byte("nameserver "))
		buffer.Write([]byte(rancherNameserver))
		buffer.Write([]byte("\n"))
	}

	input.Close()

	return ioutil.WriteFile(resolvConfLocation, buffer.Bytes(), 0666)
}
