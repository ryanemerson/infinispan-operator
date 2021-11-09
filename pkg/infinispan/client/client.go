package client

import (
	"github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	v13 "github.com/infinispan/infinispan-operator/pkg/infinispan/client/v13"
)

// New Factory to obtain Infinispan implementation. In the future this can be updated to add a REST call to determine the
// Infinispan server version, returning the api implementation required to interact with the operand.
func New(client http.HttpClient) api.Infinispan {
	return v13.New(client)
}
