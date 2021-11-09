package v13

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	"github.com/infinispan/infinispan-operator/pkg/mime"
)

const CachesPath = BasePath + "/caches"

type cache struct {
	httpClient.HttpClient
	name string
}

type caches struct {
	httpClient.HttpClient
}

func (c *cache) url() string {
	return fmt.Sprintf("%s/%s", CachesPath, c.name)
}

func (c *cache) Config(contentType mime.MimeType) (config string, err error) {
	path := c.url() + "?action=config"
	rsp, reason, err := c.Get(path, nil)
	if err = httpClient.ValidateResponse(rsp, reason, err, "getting cache config", http.StatusOK); err != nil {
		return
	}
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	var body json.RawMessage
	if err = json.NewDecoder(rsp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("unable to decode: %w", err)
	}
	return string(body), nil
}

func (c *cache) Create(config string, contentType mime.MimeType) (err error) {
	headers := map[string]string{
		"Content-Type": string(contentType),
	}

	rsp, reason, err := c.Post(c.url(), config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "creating cache", http.StatusOK)
	return
}

func (c *cache) CreateWithTemplate(templateName string) (err error) {
	path := fmt.Sprintf("%s?template=%s", c.url(), templateName)
	rsp, reason, err := c.Post(path, "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "creating cache with template", http.StatusOK)
	return
}

func (c *cache) Delete() (err error) {
	rsp, reason, err := c.HttpClient.Delete(c.url(), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "deleting cache", http.StatusOK, http.StatusNotFound)
	return
}

func (c *cache) Exists() (exist bool, err error) {
	rsp, reason, err := c.Head(c.url(), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	if err = httpClient.ValidateResponse(rsp, reason, err, "validating cache exists", http.StatusOK, http.StatusNoContent, http.StatusNotFound); err != nil {
		return
	}

	switch rsp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return
	}
	return
}

func (c *cache) RollingUpgrade() api.RollingUpgrade {
	return &rollingUpgrade{
		cache:      c,
		HttpClient: c.HttpClient,
	}
}

func (c *cache) UpdateConfig(config string, contentType mime.MimeType) (err error) {
	headers := map[string]string{
		"Content-Type": string(contentType),
	}

	rsp, reason, err := c.Put(c.url(), config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "updating cache", http.StatusOK)
	return
}

func (c *caches) ConvertConfiguration(config string, contentType mime.MimeType, reqType mime.MimeType) (transformed string, err error) {
	path := CachesPath + "?action=convert"
	headers := map[string]string{
		"Accept":       string(reqType),
		"Content-Type": string(contentType),
	}
	rsp, reason, err := c.Post(path, config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "creating cache with template", http.StatusOK)
	if err != nil {
		return
	}
	responseBody, responseErr := ioutil.ReadAll(rsp.Body)
	if responseErr != nil {
		return "", fmt.Errorf("unable to read response body: %w", responseErr)
	}
	return string(responseBody), nil
}

func (c *caches) Names() (names []string, err error) {
	rsp, reason, err := c.Get(CachesPath, nil)
	if err = httpClient.ValidateResponse(rsp, reason, err, "getting caches", http.StatusOK); err != nil {
		return
	}

	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err := json.NewDecoder(rsp.Body).Decode(&names); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	return
}
