package context

import (
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	appsv1 "k8s.io/api/apps/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

type resources struct {
	*impl
}

func (i *impl) Resources() pipeline.Resources {
	return &resources{i}
}

func (r resources) Secrets() pipeline.Secrets {
	return secrets(r)
}

func (r resources) ConfigMaps() pipeline.ConfigMaps {
	return configmaps(r)
}

func (r resources) StatefulSets() pipeline.StatefulSets {
	return statefulsets(r)
}

type secrets resources

func (s secrets) Get(name string) *corev1.Secret {
	if secret, ok := s.secrets[name]; ok {
		return secret.Definition()
	}
	return nil
}

func (s secrets) Define(secretSpec *corev1.Secret) {
	s.secrets[secretSpec.Name] = &secretResource{
		resource: secretSpec,
	}
}

func (s secrets) Load(name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := s.LoadResource(name, secret); err != nil {
		return nil, err
	}
	s.secrets[name] = &secretResource{
		persistedResource: secret,
		userCreated:       true,
	}
	return secret, nil
}

type secretResource struct {
	resource          *corev1.Secret
	persistedResource *corev1.Secret
	userCreated       bool
}

func (s *secretResource) Object() client.Object {
	return s.Definition()
}

func (s *secretResource) IsUpdated() bool {
	return !reflect.DeepEqual(s.persistedResource, s.resource)
}

func (s *secretResource) IsUserCreated() bool {
	return s.userCreated
}

func (s *secretResource) Definition() *corev1.Secret {
	if s.resource == nil {
		s.resource = s.persistedResource.DeepCopy()
	}
	return s.resource
}

type configmaps resources

func (c configmaps) Get(name string) *corev1.ConfigMap {
	if configmap, ok := c.configmaps[name]; ok {
		return configmap.Definition()
	}
	return nil
}

func (c configmaps) Define(configmap *corev1.ConfigMap) {
	c.configmaps[configmap.Name] = &configmapResource{
		resource: configmap,
	}
}

func (c configmaps) Load(name string) (*corev1.ConfigMap, error) {
	configmap := &corev1.ConfigMap{}
	if err := c.LoadResource(name, configmap); err != nil {
		return nil, err
	}
	c.configmaps[name] = &configmapResource{
		persistedResource: configmap,
		userCreated:       true,
	}
	return configmap, nil
}

type configmapResource struct {
	resource          *corev1.ConfigMap
	persistedResource *corev1.ConfigMap
	userCreated       bool
}

func (c configmapResource) Object() client.Object {
	return c.Definition()
}

func (c configmapResource) IsUserCreated() bool {
	return c.userCreated
}

func (c configmapResource) IsUpdated() bool {
	return !reflect.DeepEqual(c.persistedResource, c.resource)
}

func (c configmapResource) Definition() *corev1.ConfigMap {
	if c.resource == nil {
		c.resource = c.persistedResource.DeepCopy()
	}
	return c.resource
}

type statefulsets resources

func (s statefulsets) Get(name string) *appsv1.StatefulSet {
	if statefulset, ok := s.statefulSet[name]; ok {
		return statefulset.Definition()
	}
	return nil
}

func (s statefulsets) Define(statefulset *appsv1.StatefulSet) {
	if ss, exists := s.statefulSet[statefulset.Name]; exists {
		ss.resource = statefulset
	} else {
		s.statefulSet[statefulset.Name] = &statefulsetResource{
			resource: statefulset,
		}
	}
}

func (s statefulsets) Load(name string) (*appsv1.StatefulSet, error) {
	statefulset := &appsv1.StatefulSet{}
	if err := s.LoadResource(name, statefulset); err != nil {
		return nil, err
	}
	s.statefulSet[name] = &statefulsetResource{
		persistedResource: statefulset,
		userCreated:       true,
	}
	return statefulset, nil
}

type statefulsetResource struct {
	resource          *appsv1.StatefulSet
	persistedResource *appsv1.StatefulSet
	userCreated       bool
}

func (s statefulsetResource) Object() client.Object {
	return s.Definition()
}

func (s statefulsetResource) IsUpdated() bool {
	return !reflect.DeepEqual(s.persistedResource, s.resource)
}

func (s statefulsetResource) IsUserCreated() bool {
	return s.userCreated
}

func (s statefulsetResource) Definition() *appsv1.StatefulSet {
	if s.resource == nil {
		s.resource = s.persistedResource.DeepCopy()
	}
	return s.resource
}
