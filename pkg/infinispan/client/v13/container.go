package v13

import (
	"encoding/json"
	"fmt"
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
)

const (
	CacheManagerPath = BasePath + "/cache-managers/default"
	HealthPath       = CacheManagerPath + "/health"
)

type container struct {
	httpClient.HttpClient
}

func (c *container) Info() (info *api.ContainerInfo, error error) {
	rsp, reason, err := c.Get(CacheManagerPath, nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, reason, err, "getting cache manager info", http.StatusOK); err != nil {
		return
	}

	if err = json.NewDecoder(rsp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	return
}

func (c *container) Members() (members []string, err error) {
	rsp, reason, err := c.Get(HealthPath, nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, reason, err, "getting cluster members", http.StatusOK); err != nil {
		return
	}

	type Health struct {
		ClusterHealth struct {
			Nodes []string `json:"node_names"`
		} `json:"cluster_health"`
	}

	var health Health
	if err := json.NewDecoder(rsp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	return health.ClusterHealth.Nodes, nil
}

func (c *container) Backups() api.Backups {
	return &backups{c.HttpClient}
}

func (c *container) Restores() api.Restores {
	return &restores{c.HttpClient}
}

func (c *container) Xsite() api.Xsite {
	return &xsite{c.HttpClient}
}
