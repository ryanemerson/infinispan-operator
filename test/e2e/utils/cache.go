package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"k8s.io/apimachinery/pkg/util/wait"
)

func CacheURL(cacheName, hostAddr, key string) string {
	base := fmt.Sprintf("%v/rest/v2/caches/%s", hostAddr, cacheName)
	if key != "" {
		return fmt.Sprintf("%s/%s", base, key)
	}
	return base
}

func CacheCreateWithYaml(cacheName, hostAddr, payload string, client HTTPClient) {
	createCache(cacheName, hostAddr, payload, map[string]string{"Content-Type": "application/yaml"}, client)
}

func CacheCreateWithJSON(cacheName, hostAddr, payload string, client HTTPClient) {
	createCache(cacheName, hostAddr, payload, map[string]string{"Content-Type": "application/json"}, client)
}

func CacheCreateWithXML(cacheName, hostAddr, template string, client HTTPClient) {
	createCache(cacheName, hostAddr, template, map[string]string{"Content-Type": "application/xml;charset=UTF-8"}, client)
}

func CacheCreateWithDefault(cacheName, hostAddr string, flags string, client HTTPClient) {
	headers := map[string]string{}
	if flags != "" {
		headers["Flags"] = flags
	}
	createCache(cacheName, hostAddr, "", headers, client)
}

func createCache(cacheName, hostAddr, payload string, headers map[string]string, client HTTPClient) {
	httpURL := CacheURL(cacheName, hostAddr, "")
	resp, err := client.Post(httpURL, payload, headers)
	ExpectNoError(err)
	// Accept all the 2xx success codes
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		ThrowHTTPError(resp)
	}
}

func CacheUpdateWithYaml(cacheName, hostAddr, payload string, client HTTPClient) {
	updateCache(cacheName, hostAddr, payload, map[string]string{"Content-Type": "application/yaml"}, client)
}

func CacheUpdateWithJSON(cacheName, hostAddr, payload string, client HTTPClient) {
	updateCache(cacheName, hostAddr, payload, map[string]string{"Content-Type": "application/json"}, client)
}

func CacheUpdateWithXml(cacheName, hostAddr, payload string, client HTTPClient) {
	updateCache(cacheName, hostAddr, payload, map[string]string{"Content-Type": "application/xml;charset=UTF-8"}, client)
}

func updateCache(cacheName, hostAddr, payload string, headers map[string]string, client HTTPClient) {
	httpURL := CacheURL(cacheName, hostAddr, "")
	resp, err := client.Put(httpURL, payload, headers)
	ExpectNoError(err)
	// Accept all the 2xx success codes
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		ThrowHTTPError(resp)
	}
}

func CacheBasicUsageTest(key, value, cacheName, hostAddr string, client HTTPClient) {
	CachePutViaRoute(cacheName, hostAddr, key, value, client)
	actual := CacheGetViaRoute(cacheName, hostAddr, key, client)

	if actual != value {
		panic(fmt.Errorf("unexpected actual returned: %v (value %v)", actual, value))
	}
}

func DeleteCache(cacheName, hostAddr string, client HTTPClient) {
	httpURL := CacheURL(cacheName, hostAddr, "")
	resp, err := client.Delete(httpURL, nil)
	ExpectNoError(err)

	if resp.StatusCode != http.StatusOK {
		panic(HttpError{resp.StatusCode})
	}
}

func CacheGetViaRoute(cacheName, hostAddr, key string, client HTTPClient) string {
	url := CacheURL(cacheName, hostAddr, key)
	resp, err := client.Get(url, nil)
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

func CachePutViaRoute(cacheName, hostAddr, key, value string, client HTTPClient) {
	url := CacheURL(cacheName, hostAddr, key)
	headers := map[string]string{
		"Content-Type": "text/plain",
	}
	resp, err := client.Post(url, value, headers)
	defer CloseHttpResponse(resp)
	ExpectNoError(err)
	if resp.StatusCode != http.StatusNoContent {
		ThrowHTTPError(resp)
	}
}

func WaitForCacheToBeCreated(cacheName, hostAddr string, client HTTPClient) {
	err := wait.Poll(DefaultPollPeriod, MaxWaitTimeout, func() (done bool, err error) {
		httpURL := CacheURL(cacheName, hostAddr, "")
		fmt.Printf("Waiting for cache to be created")
		resp, err := client.Get(httpURL, nil)
		if err != nil {
			return false, err
		}
		return resp.StatusCode == http.StatusOK, nil
	})
	ExpectNoError(err)
}
