package backup

import (
	"context"
	"fmt"

	v2 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v2alpha1"
	ispnctrl "github.com/infinispan/infinispan-operator/pkg/controller/infinispan"
	zero "github.com/infinispan/infinispan-operator/pkg/controller/zerocapacity"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/backup"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	return &backupResource{instance, r.Client}, nil
}

func (r *reconcileBackup) Type() runtime.Object {
	return &v2.Backup{}
}

func (r *backupResource) AsMeta() metav1.Object {
	return r.instance
}

func (r *backupResource) Spec() *zero.Spec {
	spec := r.instance.Spec
	return &zero.Spec{
		Cluster: spec.Cluster,
		Volume: zero.VolumeSpec{
			InitPvc:      true,
			MountPath:    DataMountPath,
			SubPath:      spec.Volume.SubPath,
			VolumeSource: spec.Volume.VolumeSource,
		},
		Container: spec.Container,
		PodLabels: PodLabels(r.instance.Name, spec.Cluster),
	}
}

func (r *backupResource) Phase() zero.Phase {
	return zero.Phase(string(r.instance.Status.Phase))
}

func (r *backupResource) UpdatePhase(phase zero.Phase) error {
	instance := r.instance
	instance.Status.Phase = v2.BackupPhase(string(phase))
	err := r.client.Status().Update(ctx, instance)
	if err != nil {
		return fmt.Errorf("Failed to update Backup status: %w", err)
	}
	return nil
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
