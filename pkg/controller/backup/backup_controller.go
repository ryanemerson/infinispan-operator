package backup

import (
	"context"
	"fmt"

	v2 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v2alpha1"
	"github.com/infinispan/infinispan-operator/pkg/controller/constants"
	ispnctrl "github.com/infinispan/infinispan-operator/pkg/controller/infinispan"
	zero "github.com/infinispan/infinispan-operator/pkg/controller/zerocapacity"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/backup"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/http"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	ControllerName = "backup-controller"
	DataMountPath  = "/opt/infinispan/backups"
)

var ctx = context.Background()

// reconcileBackup reconciles a Backup object
type reconcileBackup struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
}

type backupResource struct {
	instance *v2.Backup
	client   client.Client
	scheme   *runtime.Scheme
}

func Add(mgr manager.Manager) error {
	return zero.CreateController(ControllerName, &reconcileBackup{mgr.GetClient()}, mgr)
}

func (r *reconcileBackup) ResourceInstance(key client.ObjectKey, ctrl *zero.Controller) (zero.Resource, error) {
	instance := &v2.Backup{}
	if err := ctrl.Get(ctx, key, instance); err != nil {
		return nil, err
	}

	instance.Spec.ApplyDefaults()
	return &backupResource{
		instance: instance,
		client:   r.Client,
		scheme:   ctrl.Scheme,
	}, nil
}

func (r *reconcileBackup) Type() runtime.Object {
	return &v2.Backup{}
}

func (r *backupResource) AsMeta() metav1.Object {
	return r.instance
}

func (r *backupResource) Cluster() string {
	return r.instance.Spec.Cluster
}

func (r *backupResource) Phase() zero.Phase {
	return zero.Phase(string(r.instance.Status.Phase))
}

func (r *backupResource) UpdatePhase(phase zero.Phase) error {
	instance := r.instance
	instance.Status.Phase = v2.BackupPhase(string(phase))
	return r.updateStatus()
}

func (r *backupResource) Init() (*zero.Spec, error) {
	var err error
	// TODO add labels
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.instance.Name,
			Namespace: r.instance.Namespace,
		},
	}

	volumeSpec := r.instance.Spec.Volume
	var storage resource.Quantity
	if volumeSpec.Storage == nil {
		// TODO calculate based upon number of Pods in cluster
		// ISPN- Utilise backup size estimate
		storage = constants.DefaultPVSize
	} else {
		storage, err = resource.ParseQuantity(*volumeSpec.Storage)
		if err != nil {
			return nil, err
		}
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.client, pvc, func() error {
		pvc.Spec = corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storage,
				},
			},
			StorageClassName: volumeSpec.StorageClassName,
		}
		return controllerutil.SetControllerReference(r.instance, pvc, r.scheme)
	})

	if err != nil {
		return nil, fmt.Errorf("Unable to create pvc: %w", err)
	}

	pvcName := r.instance.Name
	r.instance.Status.PVC = pvcName
	if err = r.updateStatus(); err != nil {
		return nil, err
	}

	return &zero.Spec{
		Volume: zero.VolumeSpec{
			MountPath: DataMountPath,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
		Container: r.instance.Spec.Container,
		PodLabels: PodLabels(r.instance.Name, r.instance.Spec.Cluster),
	}, nil
}

func (r *backupResource) Exec(client http.HttpClient) error {
	instance := r.instance
	backupManager := backup.NewManager(instance.Name, client)
	config := &backup.BackupConfig{
		Directory: DataMountPath,
		Resources: backup.Resources(instance.Spec.Resources),
	}
	return backupManager.Backup(instance.Name, config)
}

func (r *backupResource) ExecStatus(client http.HttpClient) (zero.Phase, error) {
	name := r.instance.Name
	backupManager := backup.NewManager(name, client)

	status, err := backupManager.BackupStatus(name)
	if err != nil {
		return zero.ZeroUnknown, fmt.Errorf("Unable to retrieve Backup status: %w", err)
	}
	return zero.Phase(status), nil
}

func PodLabels(backup, cluster string) map[string]string {
	m := ispnctrl.ServiceLabels(cluster)
	m["backup_cr"] = backup
	return m
}

func (r *backupResource) updateStatus() error {
	if err := r.client.Status().Update(ctx, r.instance); err != nil {
		return fmt.Errorf("Failed to update Backup status: %w", err)
	}
	return nil
}
