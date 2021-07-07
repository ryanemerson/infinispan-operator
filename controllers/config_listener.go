package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *infinispanRequest) ReconcileConfigListener() error {
	name := r.infinispan.GetConfigListenerName()
	namespace := r.infinispan.Namespace
	deployment := &appsv1.Deployment{}
	err := r.Client.Get(r.ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment)

	// The Deployment already exists, do nothing
	if err == nil {
		return nil
	}

	// Unknown state, return err
	if !errors.IsNotFound(err) {
		return err
	}

	// TODO mount authentication secret
	// The deployment doesn't exist, create it
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "listener",
							Image: r.infinispan.ImageName(),
							Args: []string{
								"listener",
								"--namespace",
								namespace,
								"--service-name",
								// r.infinispan.GetAdminServiceName(),
								r.infinispan.GetServiceName(), // TODO remove. Use for dev to allow access without auth
							},
						},
					},
				},
			},
		},
	}
	return r.Client.Create(r.ctx, deployment)
}

func (r *infinispanRequest) DeleteConfigListener() error {
	return r.Client.Delete(r.ctx,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.infinispan.GetConfigListenerName(),
				Namespace: r.infinispan.Namespace,
			},
		})
}
