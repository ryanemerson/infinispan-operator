package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"k8s.io/apimachinery/pkg/util/wait"
)

type CacheHelper struct {
	client    HTTPClient
	cacheName string
	hostAddr  string
	url       string
}

func CacheURL(cacheName, hostAddr, key string) string {
	base := fmt.Sprintf("%v/rest/v2/caches/%s", hostAddr, cacheName)
	if key != "" {
		return fmt.Sprintf("%s/%s", base, key)
	}
	return base
}

func NewCacheHelper(cacheName, hostAddr string, client HTTPClient) *CacheHelper {
	return &CacheHelper{
		client:    client,
		cacheName: cacheName,
		hostAddr:  hostAddr,
		url:       CacheURL(cacheName, hostAddr, ""),
	}
}

func (c *CacheHelper) entryUrl(key string) string {
	return CacheURL(c.cacheName, c.hostAddr, key)
}

func (c *CacheHelper) CreateWithYaml(payload string) {
	c.createCache(payload, map[string]string{"Content-Type": "application/yaml"})
}

func (c *CacheHelper) CreateWithJSON(payload string) {
	c.createCache(payload, map[string]string{"Content-Type": "application/json"})
}

func (c *CacheHelper) CreateWithXML(payload string) {
	c.createCache(payload, map[string]string{"Content-Type": "application/yaml"})
}

func (c *CacheHelper) CreateWithDefault(flags string) {
	headers := map[string]string{}
	if flags != "" {
		headers["Flags"] = flags
	}
	c.createCache("", headers)
}

func (c *CacheHelper) createCache(payload string, headers map[string]string) {
	resp, err := c.client.Post(c.url, payload, headers)
	ExpectNoError(err)
	// Accept all the 2xx success codes
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		ThrowHTTPError(resp)
	}
}

func (c *CacheHelper) UpdateWithYaml(payload string) {
	c.updateCache(payload, map[string]string{"Content-Type": "application/yaml"})
}

func (c *CacheHelper) UpdateWithJSON(payload string) {
	c.updateCache(payload, map[string]string{"Content-Type": "application/json"})
}

func (c *CacheHelper) UpdateWithXML(payload string) {
	c.updateCache(payload, map[string]string{"Content-Type": "application/xml"})
}

func (c *CacheHelper) updateCache(payload string, headers map[string]string) {
	resp, err := c.client.Put(c.url, payload, headers)
	ExpectNoError(err)
	// Accept all the 2xx success codes
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		ThrowHTTPError(resp)
	}
}

func (c *CacheHelper) TestBasicUsage(key, value string) {
	c.PutWithPlainText(key, value)
	actual := c.Get(key)
	if actual != value {
		panic(fmt.Errorf("unexpected actual returned: %v (value %v)", actual, value))
	}
}

func (c *CacheHelper) Delete() {
	resp, err := c.client.Delete(c.url, nil)
	ExpectNoError(err)

	if resp.StatusCode != http.StatusOK {
		panic(HttpError{resp.StatusCode})
	}
}

func (c *CacheHelper) Get(key string) string {
	resp, err := c.client.Get(c.entryUrl(key), nil)
	ExpectNoError(err)
	defer func(Body io.ReadCloser) {
		ExpectNoError(Body.Close())
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		ThrowHTTPError(resp)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	ExpectNoError(err)
	return string(bodyBytes)
}

func (c *CacheHelper) PutWithPlainText(key, value string) {
	c.Put(key, value, map[string]string{"Content-Type": "text/plain"})
}

func (c *CacheHelper) Put(key, value string, headers map[string]string) {
	resp, err := c.client.Post(c.entryUrl(key), value, headers)
	defer CloseHttpResponse(resp)
	ExpectNoError(err)
	if resp.StatusCode != http.StatusNoContent {
		ThrowHTTPError(resp)
	}
}

func (c *CacheHelper) WaitForCacheToExist() {
	err := wait.Poll(DefaultPollPeriod, MaxWaitTimeout, func() (done bool, err error) {
		fmt.Printf("Waiting for cache to be created")
		resp, err := c.client.Get(c.url, nil)
		if err != nil {
			return false, err
		}
		return resp.StatusCode == http.StatusOK, nil
	})
	ExpectNoError(err)
}
