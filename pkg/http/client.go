package http

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/infinispan/infinispan-operator/controllers/constants"
)

type HttpClient interface {
	Head(path string, headers map[string]string) (*http.Response, error)
	Get(path string, headers map[string]string) (*http.Response, error)
	Post(path, payload string, headers map[string]string) (*http.Response, error)
	Put(path, payload string, headers map[string]string) (*http.Response, error)
	Delete(path string, headers map[string]string) (*http.Response, error)
}

func ValidateResponse(rsp *http.Response, inperr error, entity string, validCodes ...int) (err error) {
	if inperr != nil {
		return fmt.Errorf("unexpected error %s: %w", entity, inperr)
	}

	if rsp == nil || len(validCodes) == 0 {
		return
	}

	for _, code := range validCodes {
		if code == rsp.StatusCode {
			return
		}
	}

	defer func() {
		cerr := rsp.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	responseBody, responseErr := ioutil.ReadAll(rsp.Body)
	if responseErr != nil {
		return fmt.Errorf("server side error %s. Unable to read response body, %w", entity, responseErr)
	}
	return fmt.Errorf("unexpected error %s, response: %v", entity, constants.GetWithDefault(string(responseBody), rsp.Status))
}

func CloseBody(rsp *http.Response, err error) error {
	var cerr error
	if rsp != nil {
		cerr = rsp.Body.Close()
	}
	if err == nil {
		return cerr
	}
	return err
}
