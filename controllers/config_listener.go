package controllers

import (
	"fmt"

	"github.com/infinispan/infinispan-operator/controllers/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *infinispanRequest) ReconcileConfigListener() error {
	if constants.ConfigListenerImageName == "" {
		err := fmt.Errorf("'%s' has not been defined", constants.ConfigListenerEnvName)
		r.log.Error(err, "unable to create ConfigListener deployment")
		return nil
	}
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
	labels := ConfigListenerPodLabels(r.infinispan.Name)
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "listener",
							Image: constants.ConfigListenerImageName,
							Args: []string{
								"listener",
								"-namespace",
								namespace,
								"-cluster",
								r.infinispan.Name,
							},
						},
					},
				},
			},
		},
	}
	_, err = controllerutil.CreateOrUpdate(r.ctx, r.Client, deployment, func() error {
		return controllerutil.SetControllerReference(r.infinispan, deployment, r.scheme)
	})
	return err
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
