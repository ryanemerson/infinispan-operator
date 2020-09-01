package restore

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
	ControllerName = "restore-controller"
	DataMountPath  = "/opt/infinispan/restores"
)

var ctx = context.Background()

// ReconcileRestore reconciles a Restore object
type reconcileRestore struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
}

type restore struct {
	instance *v2.Restore
	client   client.Client
}

func Add(mgr manager.Manager) error {
	return zero.CreateController(ControllerName, &reconcileRestore{mgr.GetClient()}, mgr)
}

func (r *reconcileRestore) ResourceInstance(key client.ObjectKey, ctrl *zero.Controller) (zero.Resource, error) {
	instance := &v2.Restore{}
	if err := ctrl.Get(ctx, key, instance); err != nil {
		return nil, err
	}

	instance.ApplyDefaults()
	return &restore{instance, r.Client}, nil
}

func (r *reconcileRestore) Type() runtime.Object {
	return &v2.Restore{}
}

func (r *restore) AsMeta() metav1.Object {
	return r.instance
}

func (r *restore) Spec() *zero.Spec {
	spec := r.instance.Spec
	return &zero.Spec{
		Cluster: spec.Cluster,
		Volume: zero.VolumeSpec{
			MountPath:    DataMountPath,
			SubPath:      spec.Volume.SubPath,
			VolumeSource: spec.Volume.VolumeSource,
		},
		Container: spec.Container,
		PodLabels: PodLabels(r.instance.Name, spec.Cluster),
	}
}

func (r *restore) Phase() zero.Phase {
	return zero.Phase(string(r.instance.Status.Phase))
}

func (r *restore) UpdatePhase(phase zero.Phase) error {
	instance := r.instance
	instance.Status.Phase = v2.RestorePhase(string(phase))
	err := r.client.Status().Update(ctx, instance)
	if err != nil {
		return fmt.Errorf("Failed to update Backup status: %w", err)
	}
	return nil
}

func (r *restore) Exec(client http.HttpClient) error {
	instance := r.instance
	backupManager := backup.NewManager(instance.Name, client)
	config := &backup.RestoreConfig{
		Location:  fmt.Sprintf("%[1]s/%[2]s/%[2]s.zip", DataMountPath, instance.Spec.BackupName),
		Resources: backup.Resources(instance.Spec.Resources),
	}
	return backupManager.Restore(instance.Name, config)
}

func (r *restore) ExecStatus(client http.HttpClient) (zero.Phase, error) {
	name := r.instance.Name
	backupManager := backup.NewManager(name, client)

	status, err := backupManager.RestoreStatus(name)
	if err != nil {
		return zero.ZeroUnknown, fmt.Errorf("Unable to retrieve Restore status: %w", err)
	}
	return zero.Phase(status), nil
}

func PodLabels(backup, cluster string) map[string]string {
	m := ispnctrl.ServiceLabels(cluster)
	m["restore_cr"] = backup
	return m
}
