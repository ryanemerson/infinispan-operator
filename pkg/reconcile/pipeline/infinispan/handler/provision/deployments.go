package provision

import (
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/api/v2alpha1"
	"github.com/infinispan/infinispan-operator/controllers/constants"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConfigListener(ctx pipeline.Context) {
	i := ctx.Instance()
	r := ctx.Resources()
	name := i.GetConfigListenerName()

	objectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: i.Namespace,
	}

	if !i.IsConfigListenerEnabled() {
		// Mark any previously created ConfigListener resources for removal and continue pipeline execution
		r.MarkForDeletion(&appsv1.Deployment{ObjectMeta: objectMeta})
		r.MarkForDeletion(&rbacv1.RoleBinding{ObjectMeta: objectMeta})
		r.MarkForDeletion(&rbacv1.Role{ObjectMeta: objectMeta})
		r.MarkForDeletion(&corev1.ServiceAccount{ObjectMeta: objectMeta})
		return
	}

	// Define a ServiceAccount in the cluster namespace so that the ConfigListener has the required API permissions
	r.Define(&corev1.ServiceAccount{
		ObjectMeta: objectMeta,
	}, true)

	r.Define(&rbacv1.Role{
		ObjectMeta: objectMeta,
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{v2alpha1.GroupVersion.Group},
				Resources: []string{"caches"},
				Verbs: []string{
					"create",
					"delete",
					"get",
					"list",
					"patch",
					"update",
					"watch",
				},
			},
			{
				APIGroups: []string{ispnv1.GroupVersion.Group},
				Resources: []string{"infinispans"},
				Verbs:     []string{"get"},
			}, {
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list"},
			}, {
				APIGroups: []string{""},
				Resources: []string{"pods/exec"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
		},
	}, true)

	r.Define(&rbacv1.RoleBinding{
		ObjectMeta: objectMeta,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: i.Namespace,
		}},
	}, true)

	labels := i.PodLabels()
	labels["app"] = "infinispan-config-listener-pod"
	r.Define(&appsv1.Deployment{
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
							Name:  "infinispan-listener",
							Image: constants.ConfigListenerImageName,
							Args: []string{
								"listener",
								"-namespace",
								i.Namespace,
								"-cluster",
								i.Name,
							},
						},
					},
					ServiceAccountName: name,
				},
			},
		},
	}, true)
}
