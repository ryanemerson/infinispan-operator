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

type HttpError struct {
	Status  int
	Message string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("unexpected HTTP status code (%d): %s", e.Status, e.Message)
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
		return &HttpError{
			Status:  rsp.StatusCode,
			Message: fmt.Sprintf("server side error %s. Unable to read response body: %s", entity, responseErr.Error()),
		}
	}
	return &HttpError{
		Status:  rsp.StatusCode,
		Message: fmt.Sprintf("unexpected error %s, response: %s", entity, constants.GetWithDefault(string(responseBody), rsp.Status)),
	}
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
