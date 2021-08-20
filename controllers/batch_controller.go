package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "github.com/infinispan/infinispan-operator/api/v1"
	v2 "github.com/infinispan/infinispan-operator/api/v2alpha1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	"github.com/prometheus/common/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	BatchFilename   = "batch"
	BatchVolumeName = "batch-volume"
	BatchVolumeRoot = "/etc/batch"
)

// BatchReconciler reconciles a Batch object
type BatchReconciler struct {
	client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	kubernetes *kube.Kubernetes
	eventRec   record.EventRecorder
}

// Struct for wrapping reconcile request data
type batchRequest struct {
	*BatchReconciler
	ctx       context.Context
	req       ctrl.Request
	batch     *v2.Batch
	reqLogger logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func (r *BatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.log = ctrl.Log.WithName("controllers").WithName("Batch")
	r.scheme = mgr.GetScheme()
	r.kubernetes = kube.NewKubernetesFromController(mgr)
	r.eventRec = mgr.GetEventRecorderFor("batch-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.Batch{}).Owns(&batchv1.Job{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infinispan.org,resources=batches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infinispan.org,resources=batches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infinispan.org,resources=batches/finalizers,verbs=update

func (reconciler *BatchReconciler) Reconcile(ctx context.Context, ctrlRequest ctrl.Request) (ctrl.Result, error) {
	reqLogger := reconciler.log.WithValues("Request.Namespace", ctrlRequest.Namespace, "Request.Name", ctrlRequest.Name)
	reqLogger.Info("Reconciling Batch")

	// Fetch the Batch instance
	instance := &v2.Batch{}
	err := reconciler.Get(ctx, ctrlRequest.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	batch := &batchRequest{
		BatchReconciler: reconciler,
		ctx:             ctx,
		req:             ctrlRequest,
		batch:           instance,
		reqLogger:       reqLogger,
	}

	phase := instance.Status.Phase
	switch phase {
	case "":
		return batch.validate()
	case v2.BatchInitializing:
		return batch.initializeResources()
	case v2.BatchInitialized:
		return batch.execute()
	case v2.BatchRunning:
		return batch.waitToComplete()
	default:
		// Backup either succeeded or failed
		return ctrl.Result{}, nil
	}
}

func (r *batchRequest) validate() (reconcile.Result, error) {
	spec := r.batch.Spec

	if spec.ConfigMap == nil && spec.Config == nil {
		return reconcile.Result{},
			r.UpdatePhase(v2.BatchFailed, fmt.Errorf("'Spec.config' OR 'spec.ConfigMap' must be configured"))
	}

	if spec.ConfigMap != nil && spec.Config != nil {
		return reconcile.Result{},
			r.UpdatePhase(v2.BatchFailed, fmt.Errorf("at most one of ['Spec.config', 'spec.ConfigMap'] must be configured"))
	}
	return reconcile.Result{}, r.UpdatePhase(v2.BatchInitializing, nil)
}

func (r *batchRequest) initializeResources() (reconcile.Result, error) {
	batch := r.batch
	spec := batch.Spec
	// Ensure the Infinispan cluster exists
	infinispan := &v1.Infinispan{}
	if result, err := kube.LookupResource(spec.Cluster, batch.Namespace, infinispan, r.Client, r.reqLogger, r.eventRec); result != nil {
		return *result, err
	}

	if err := infinispan.EnsureClusterStability(); err != nil {
		log.Info(fmt.Sprintf("Infinispan '%s' not ready: %s", spec.Cluster, err.Error()))
		return reconcile.Result{RequeueAfter: consts.DefaultWaitOnCluster}, nil
	}

	if spec.ConfigMap == nil {
		// Create configMap
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      batch.Name,
				Namespace: batch.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(r.ctx, r.Client, configMap, func() error {
			configMap.Data = map[string]string{BatchFilename: *spec.Config}
			return controllerutil.SetControllerReference(batch, configMap, r.scheme)
		})

		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to create ConfigMap '%s': %w", configMap.Name, err)
		}

		updated, err := r.update(func() error {
			batch.Spec.ConfigMap = &batch.Name
			return nil
		})
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to update Batch .Spec: %w", err)
		}
		if updated {
			return reconcile.Result{}, nil
		}
	}

	// We update the phase separately to the spec as the status update is ignored when in the update mutate function
	_, err := r.update(func() error {
		batch.Status.ClusterUID = &infinispan.UID
		batch.Status.Phase = v2.BatchInitialized
		return nil
	})
	return reconcile.Result{}, err
}

func (r *batchRequest) execute() (reconcile.Result, error) {
	batch := r.batch
	infinispan := &v1.Infinispan{}
	if result, err := kube.LookupResource(batch.Spec.Cluster, batch.Namespace, infinispan, r.Client, r.reqLogger, r.eventRec); result != nil {
		return *result, err
	}

	expectedUid := *batch.Status.ClusterUID
	if infinispan.GetUID() != expectedUid {
		err := fmt.Errorf("unable to execute Batch. Infinispan CR UUID has changed, expected '%s' observed '%s'", expectedUid, infinispan.GetUID())
		return reconcile.Result{}, r.UpdatePhase(v2.BatchFailed, err)
	}

	cliArgs := fmt.Sprintf("--properties '%s/%s' --file '%s/%s'", consts.ServerAdminIdentitiesRoot, consts.CliPropertiesFilename, BatchVolumeRoot, BatchFilename)

	labels := BatchLabels(batch.Name)
	infinispan.AddLabelsForPods(labels)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      batch.Name,
			Namespace: batch.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointer.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    batch.Name,
						Image:   infinispan.ImageName(),
						Command: []string{"/opt/infinispan/bin/cli.sh", cliArgs},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      BatchVolumeName,
								MountPath: BatchVolumeRoot,
							},
							{
								Name:      AdminIdentitiesVolumeName,
								MountPath: consts.ServerAdminIdentitiesRoot,
							},
						},
					}},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						// Volume for Batch ConfigMap
						{
							Name: BatchVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: *batch.Spec.ConfigMap},
								},
							},
						},
						// Volume for cli.properties
						{
							Name: AdminIdentitiesVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: infinispan.GetAdminSecretName(),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := controllerutil.CreateOrUpdate(r.ctx, r.Client, job, func() error {
		return controllerutil.SetControllerReference(batch, job, r.scheme)
	})

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to create batch job '%s': %w", batch.Name, err)
	}
	return reconcile.Result{}, r.UpdatePhase(v2.BatchRunning, nil)
}

func (r *batchRequest) waitToComplete() (reconcile.Result, error) {
	batch := r.batch
	job := &batchv1.Job{}
	if result, err := kube.LookupResource(batch.Name, batch.Namespace, job, r.Client, r.reqLogger, r.eventRec); result != nil {
		return *result, err
	}

	status := job.Status
	if status.Succeeded > 0 {
		return reconcile.Result{}, r.UpdatePhase(v2.BatchSucceeded, nil)
	}

	if status.Failed > 0 {
		numConditions := len(status.Conditions)
		if numConditions > 0 {
			condition := status.Conditions[numConditions-1]

			if condition.Type == batchv1.JobFailed {
				var reason string
				podName, err := GetJobPodName(batch.Name, batch.Namespace, r.Client, r.ctx)
				if err != nil {
					reason = err.Error()
				} else {
					reason, err = r.kubernetes.Logs(podName, batch.Namespace)
					if err != nil {
						reason = fmt.Errorf("unable to retrive logs for batch %s: %w", batch.Name, err).Error()
					}
				}

				_, err = r.update(func() error {
					r.batch.Status.Phase = v2.BatchFailed
					r.batch.Status.Reason = reason
					return nil
				})
				return reconcile.Result{}, err
			}
		}
	}
	// The job has not completed, wait 1 second before retrying
	return reconcile.Result{}, nil
}

func (r *batchRequest) UpdatePhase(phase v2.BatchPhase, phaseErr error) error {
	_, err := r.update(func() error {
		batch := r.batch
		var reason string
		if phaseErr != nil {
			reason = phaseErr.Error()
		}
		batch.Status.Phase = phase
		batch.Status.Reason = reason
		return nil
	})
	return err
}

func (r *batchRequest) update(mutate func() error) (bool, error) {
	batch := r.batch
	res, err := controllerutil.CreateOrPatch(r.ctx, r.Client, batch, func() error {
		if batch.CreationTimestamp.IsZero() {
			return errors.NewNotFound(schema.ParseGroupResource("batch.infinispan.org"), batch.Name)
		}
		return mutate()
	})
	return res != controllerutil.OperationResultNone, err
}

func BatchLabels(name string) map[string]string {
	return map[string]string{
		"infinispan_batch": name,
		"app":              "infinispan-batch-pod",
	}
}

func GetJobPodName(name, namespace string, c client.Client, ctx context.Context) (string, error) {
	labelSelector := labels.SelectorFromSet(BatchLabels(name))
	podList := &corev1.PodList{}
	listOps := &client.ListOptions{Namespace: namespace, LabelSelector: labelSelector}
	if err := c.List(ctx, podList, listOps); err != nil {
		return "", fmt.Errorf("unable to retrieve pod name for batch %s: %w", name, err)
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no Batch job pods found")
	}
	return podList.Items[0].Name, nil
}
