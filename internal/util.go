package internal

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
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

// PostJSON posts json object to url
func PostJSON(url string, obj interface{}, tlsConfig *tls.Config) (*http.Response, error) {
	var client *http.Client
	if tlsConfig == nil {
		client = &http.Client{}
	} else {
		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		client = &http.Client{
			Transport: tr,
		}
	}

	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(obj); err != nil {
		return nil, err
	}
	return client.Post(url, "application/json; charset=utf-8", b)
}

// GetJSON gets a json response from url
func GetJSON(url string, obj interface{}, tlsConfig *tls.Config) (*http.Response, error) {
	var client *http.Client
	if tlsConfig == nil {
		client = &http.Client{}
	} else {
		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		client = &http.Client{
			Transport: tr,
		}
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
