package controllers

import (
	"context"
	"fmt"

	v1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/http/curl"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	users "github.com/infinispan/infinispan-operator/pkg/infinispan/security"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
)

func NewInfinispan(ctx context.Context, i *v1.Infinispan, kubernetes *kube.Kubernetes) (api.Infinispan, error) {
	podList, err := PodsCreatedBy(i.Namespace, kubernetes, ctx, i.GetStatefulSetName())
	if err != nil {
		return nil, err
	}
	return NewInfinispanForPod(ctx, podList.Items[0].Name, i, kubernetes)
}

func NewInfinispanForPod(ctx context.Context, podName string, i *v1.Infinispan, kubernetes *kube.Kubernetes) (api.Infinispan, error) {
	curl, err := NewCurlClient(ctx, podName, i, kubernetes)
	if err != nil {
		return nil, fmt.Errorf("unable to create Infinispan client: %w", err)
	}
	return client.New(curl), nil
}

func NewCurlClient(ctx context.Context, podName string, i *v1.Infinispan, kubernetes *kube.Kubernetes) (*curl.Client, error) {
	pass, err := users.AdminPassword(i.GetAdminSecretName(), i.Namespace, kubernetes, ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve operator admin identities when creating Curl client: %w", err)
	}
	curlClient := curl.New(curl.Config{
		Credentials: &curl.Credentials{
			Username: consts.DefaultOperatorUser,
			Password: pass,
		},
		Podname:   podName,
		Namespace: i.Namespace,
		Protocol:  "http",
		Port:      consts.InfinispanAdminPort,
	}, kubernetes)
	return curlClient, nil
}
