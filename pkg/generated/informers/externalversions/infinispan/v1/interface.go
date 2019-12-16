// Code generated by main. DO NOT EDIT.

package v1

import (
	internalinterfaces "github.com/infinispan/infinispan-operator/pkg/generated/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Infinispans returns a InfinispanInformer.
	Infinispans() InfinispanInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// Infinispans returns a InfinispanInformer.
func (v *version) Infinispans() InfinispanInformer {
	return &infinispanInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
