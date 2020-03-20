package internal

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"
)

// GetTLSConfig generate tls.Config from CAFile
func GetTLSConfig(CAFile string) (*tls.Config, error) {
	caCert, err := ioutil.ReadFile(CAFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, errors.New("Failed to add CA to pool")
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	return tlsConfig, nil
}

// CreateHTTPClient returns a http.Client
func CreateHTTPClient(CAFile string) (*http.Client, error) {
	var tlsConfig *tls.Config
	var err error

	if CAFile != "" {
		tlsConfig, err = GetTLSConfig(CAFile)
		if err != nil {
			return nil, err
		}
	}

	tr := &http.Transport{
		MaxIdleConnsPerHost: 20,
		TLSClientConfig:     tlsConfig,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   5 * time.Second,
	}, nil
}

// PostJSON posts json object to url
func PostJSON(url string, obj interface{}, client *http.Client) (*http.Response, error) {
	if client == nil {
		client, _ = CreateHTTPClient("")
	}
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(obj); err != nil {
		return nil, err
	}
	return client.Post(url, "application/json; charset=utf-8", b)
}

// GetJSON gets a json response from url
func GetJSON(url string, obj interface{}, client *http.Client) (*http.Response, error) {
	if client == nil {
		client, _ = CreateHTTPClient("")
	}

	resp, err := client.Get(url)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusOK {
		return resp, errors.New("HTTP status code is not 200")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(body, obj)
}

// FindAllSubmatchInFile calls re.FindAllSubmatch to find matches in given file
func FindAllSubmatchInFile(fileName string, re *regexp.Regexp) (matches [][][]byte, err error) {
	if fileName == "/dev/null" {
		err = errors.New("Invalid log file")
		return
	}
	if content, err := ioutil.ReadFile(fileName); err == nil {
		matches = re.FindAllSubmatch(content, -1)
		// fmt.Printf("FindAllSubmatchInFile: %q\n", matches)
	}
	return
}

// ExtractSizeFromLog uses a regexp to extract the size from log files
func ExtractSizeFromLog(logFile string, re *regexp.Regexp) string {
	matches, _ := FindAllSubmatchInFile(logFile, re)
	if matches == nil || len(matches) == 0 {
		return ""
	}
	// return the first capture group of the last occurrence
	return string(matches[len(matches)-1][1])
}

// ExtractSizeFromRsyncLog extracts the size from rsync logs
func ExtractSizeFromRsyncLog(logFile string) string {
	// (?m) flag enables multi-line mode
	re := regexp.MustCompile(`(?m)^Total file size: ([0-9\.]+[KMGTP]?) bytes`)
	return ExtractSizeFromLog(logFile, re)
}
