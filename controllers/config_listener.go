package controllers

import (
	"fmt"

	"github.com/infinispan/infinispan-operator/api/v2alpha1"
	"github.com/infinispan/infinispan-operator/controllers/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	objectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}

	err := r.Client.Get(r.ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment)

	// The Deployment already exists, do nothing
	if err == nil {
		return nil
	}

	setCtrlRefAndCreate := func(obj client.Object) error {
		err := controllerutil.SetControllerReference(r.infinispan, obj, r.scheme)
		if err != nil {
			return err
		}
		return r.Client.Create(r.ctx, obj)
	}

	// Create a ServiceAccount in the cluster namespace so that the ConfigListener has the required API permissions
	sa := &corev1.ServiceAccount{
		ObjectMeta: objectMeta,
	}
	if err := setCtrlRefAndCreate(sa); err != nil {
		return err
	}

	role := &rbacv1.Role{
		ObjectMeta: objectMeta,
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{v2alpha1.GroupVersion.Group},
				Resources: []string{"caches"},
				Verbs:     []string{rbacv1.VerbAll},
			},
			{
				APIGroups: []string{"", v2alpha1.GroupVersion.Group},
				Resources: []string{"infinispans", "secrets"},
				Verbs:     []string{"get"},
			},
		},
	}
	if err := setCtrlRefAndCreate(role); err != nil {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: objectMeta,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: namespace,
		}},
	}
	if err := setCtrlRefAndCreate(roleBinding); err != nil {
		return err
	}

	// TODO mount authentication secret
	// The deployment doesn't exist, create it
	labels := ConfigListenerPodLabels(r.infinispan.Name)
	// TODO how does this work with permissions?
	// Does this Deployment have the same ServiceAccount as the resource creating it?
	deployment = &appsv1.Deployment{
		ObjectMeta: objectMeta,
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
					ServiceAccountName: name,
				},
			},
		},
	}
	return setCtrlRefAndCreate(deployment)
}

func (r *infinispanRequest) DeleteConfigListener() error {
	objectMeta := metav1.ObjectMeta{
		Name:      r.infinispan.GetConfigListenerName(),
		Namespace: r.infinispan.Namespace,
	}

	delete := func(obj client.Object) error {
		err := r.Client.Delete(r.ctx, obj)
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := delete(&appsv1.Deployment{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := delete(&rbacv1.RoleBinding{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := delete(&rbacv1.Role{ObjectMeta: objectMeta}); err != nil {
		return err
	}
	return delete(&corev1.ServiceAccount{ObjectMeta: objectMeta})
}
