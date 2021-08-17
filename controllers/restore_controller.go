package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/infinispan/infinispan-operator/api/v2alpha1"
	ispnctrl "github.com/infinispan/infinispan-operator/controllers/infinispan"
	zero "github.com/infinispan/infinispan-operator/controllers/zerocapacity"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/backup"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/http"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=infinispan.org,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infinispan.org,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infinispan.org,resources=restores/finalizers,verbs=update

var (
	restoreCtx = context.Background()
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type restore struct {
	instance *v2alpha1.Restore
	client   client.Client
	scheme   *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	backupEventRec := mgr.GetEventRecorderFor(BackupControllerName)
	return zero.CreateController(BackupControllerName, &RestoreReconciler{mgr.GetClient(), mgr.GetLogger(), mgr.GetScheme()}, mgr, backupEventRec)
}

func (r *RestoreReconciler) ResourceInstance(name types.NamespacedName, ctrl *zero.Controller) (zero.Resource, error) {
	instance := &v2alpha1.Restore{}
	if err := ctrl.Get(restoreCtx, name, instance); err != nil {
		return nil, err
	}

	restore := &restore{
		instance: instance,
		client:   r.Client,
		scheme:   ctrl.Scheme,
	}
	return restore, nil
}

func (r *RestoreReconciler) Type() client.Object {
	return &v2alpha1.Restore{}
}

func (r *restore) AsMeta() metav1.Object {
	return r.instance
}

func (r *restore) Cluster() string {
	return r.instance.Spec.Cluster
}

func (r *restore) Phase() zero.Phase {
	return zero.Phase(string(r.instance.Status.Phase))
}

func (r *restore) UpdatePhase(phase zero.Phase, phaseErr error) error {
	_, err := r.update(func() {
		restore := r.instance
		var reason string
		if phaseErr != nil {
			reason = phaseErr.Error()
		}
		restore.Status.Phase = v2alpha1.RestorePhase(phase)
		restore.Status.Reason = reason
	})
	return err
}

func (r *restore) Transform() (bool, error) {
	return r.update(func() {
		restore := r.instance
		restore.Spec.ApplyDefaults()
		resources := restore.Spec.Resources
		if resources == nil {
			return
		}

		if len(resources.CacheConfigs) > 0 {
			resources.Templates = resources.CacheConfigs
			resources.CacheConfigs = nil
		}

		if len(resources.Scripts) > 0 {
			resources.Tasks = resources.Scripts
			resources.Scripts = nil
		}
	})
}

func (r *restore) update(mutate func()) (bool, error) {
	restore := r.instance
	res, err := controllerutil.CreateOrPatch(restoreCtx, r.client, restore, func() error {
		if restore.CreationTimestamp.IsZero() {
			return errors.NewNotFound(schema.ParseGroupResource("restore.infinispan.org"), restore.Name)
		}
		mutate()
		return nil
	})
	return res != controllerutil.OperationResultNone, err
}

func (r *restore) Init() (*zero.Spec, error) {
	backup := &v2alpha1.Backup{}
	backupKey := types.NamespacedName{
		Namespace: r.instance.Namespace,
		Name:      r.instance.Spec.Backup,
	}

	if err := r.client.Get(restoreCtx, backupKey, backup); err != nil {
		return nil, fmt.Errorf("Unable to load Infinispan Backup '%s': %w", backupKey.Name, err)
	}

	return &zero.Spec{
		Container: r.instance.Spec.Container,
		PodLabels: RestorePodLabels(r.instance.Name, backup.Spec.Cluster),
		Volume: zero.VolumeSpec{
			MountPath: BackupDataMountPath,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: backup.Name,
					ReadOnly:  true,
				},
			},
		},
	}, nil
}

func (r *restore) Exec(client http.HttpClient) error {
	instance := r.instance
	backupManager := backup.NewManager(instance.Name, client)
	var resources backup.Resources
	if instance.Spec.Resources == nil {
		resources = backup.Resources{}
	} else {
		resources = backup.Resources{
			Caches:       instance.Spec.Resources.Caches,
			Counters:     instance.Spec.Resources.Counters,
			ProtoSchemas: instance.Spec.Resources.ProtoSchemas,
			Templates:    instance.Spec.Resources.Templates,
			Tasks:        instance.Spec.Resources.Tasks,
		}
	}
	config := &backup.RestoreConfig{
		Location:  fmt.Sprintf("%[1]s/%[2]s/%[2]s.zip", BackupDataMountPath, instance.Spec.Backup),
		Resources: resources,
	}
	return backupManager.Restore(instance.Name, config)
}

func (r *restore) ExecStatus(client http.HttpClient) (zero.Phase, error) {
	name := r.instance.Name
	backupManager := backup.NewManager(name, client)

	status, err := backupManager.RestoreStatus(name)
	if err != nil {
		return zero.ZeroUnknown, err
	}
	return zero.Phase(status), nil
}

func RestorePodLabels(backup, cluster string) map[string]string {
	m := ispnctrl.ServiceLabels(cluster)
	m["restore_cr"] = backup
	return m
}
