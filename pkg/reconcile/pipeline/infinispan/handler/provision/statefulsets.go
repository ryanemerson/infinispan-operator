package provision

import (
	"fmt"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	consts "github.com/infinispan/infinispan-operator/controllers/constants"
	"github.com/infinispan/infinispan-operator/pkg/hash"
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"strings"
	"time"
)

const (
	InfinispanContainer          = "infinispan"
	GossipRouterContainer        = "gossiprouter"
	DataMountPath                = consts.ServerRoot + "/data"
	OperatorConfMountPath        = consts.ServerRoot + "/conf/operator"
	DataMountVolume              = "data-volume"
	ConfigVolumeName             = "config-volume"
	EncryptKeystoreVolumeName    = "encrypt-volume"
	EncryptTruststoreVolumeName  = "encrypt-trust-volume"
	IdentitiesVolumeName         = "identities-volume"
	UserConfVolumeName           = "user-conf-volume"
	InfinispanSecurityVolumeName = "infinispan-security-volume"
	OverlayConfigMountPath       = consts.ServerRoot + "/conf/user"

	EventReasonLowPersistenceStorage = "LowPersistenceStorage"
	EventReasonEphemeralStorage      = "EphemeralStorageEnables"
	EventReasonParseValueProblem     = "ParseValueProblem"
	EventLoadBalancerUnsupported     = "LoadBalancerUnsupported"

	SiteTransportKeystoreVolumeName = "encrypt-transport-site-tls-volume"
	SiteRouterKeystoreVolumeName    = "encrypt-router-site-tls-volume"
	SiteTruststoreVolumeName        = "encrypt-truststore-site-tls-volume"
)

func AddChmodInitContainer(ctx pipeline.Context) {
	i := ctx.Instance()
	statefulSet := &appsv1.StatefulSet{}
	if err := ctx.Resources().Load(i.GetStatefulSetName(), statefulSet); err != nil {
		ctx.Error(fmt.Errorf("unable to add InitContainer: %w", err))
	}
	c := &statefulSet.Spec.Template.Spec.InitContainers
	*c = append(*c, ChmodInitContainer("data-chmod-pv", DataMountVolume, DataMountPath))
}

func ClusterStatefulSet(ctx pipeline.Context) {
	i := ctx.Instance()

	labelsForPod := i.PodLabels()
	labelsForPod[consts.StatefulSetPodLabel] = i.Name

	annotationsForPod := i.PodAnnotations()
	annotationsForPod["updateDate"] = time.Now().String()

	// We can ignore the err here as the validating webhook ensures that the resources are valid
	podResources, _ := PodResources(i.Spec.Container)
	configFiles := ctx.ConfigFiles()
	statefulset := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.GetStatefulSetName(),
			Namespace: i.Namespace,
			Annotations: map[string]string{
				"openshift.io/display-name":      "Infinispan Cluster",
				"openshift.io/documentation-url": "http://infinispan.org/documentation/",
			},
			Labels: map[string]string{},
		},
		Spec: appsv1.StatefulSetSpec{
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType},
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForPod,
			},
			Replicas: &i.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labelsForPod,
					Annotations: annotationsForPod,
				},
				Spec: corev1.PodSpec{
					Affinity: i.Spec.Affinity,
					Containers: []corev1.Container{{
						Image: i.ImageName(),
						Args:  buildStartupArgs(configFiles.UserConfig.ServerConfigEncoding, configFiles.UserConfig.Log4j != ""),
						Name:  InfinispanContainer,
						Env: PodEnv(i, &[]corev1.EnvVar{
							{Name: "CONFIG_HASH", Value: hash.HashString(configFiles.ServerConfig)},
							{Name: "ADMIN_IDENTITIES_HASH", Value: hash.HashByte(configFiles.AdminIdentities.IdentitiesFile)},
							{Name: "IDENTITIES_BATCH", Value: consts.ServerOperatorSecurity + "/" + consts.ServerIdentitiesCliFilename},
						}),
						LivenessProbe:  PodLivenessProbe(),
						Ports:          PodPortsWithXsite(i),
						ReadinessProbe: PodReadinessProbe(),
						StartupProbe:   PodStartupProbe(),
						Resources:      *podResources,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      ConfigVolumeName,
							MountPath: OperatorConfMountPath,
						}, {
							Name:      InfinispanSecurityVolumeName,
							MountPath: consts.ServerOperatorSecurity,
						}, {
							Name:      DataMountVolume,
							MountPath: DataMountPath,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: ConfigVolumeName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: i.GetConfigName()},
							},
						},
					}, {
						Name: InfinispanSecurityVolumeName,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: i.GetInfinispanSecuritySecretName(),
							},
						},
					},
					},
				},
			},
		},
	}

	if err := addDataMountVolume(ctx, i, statefulset); err != nil {
		ctx.RetryProcessing(err)
		return
	}

	if _, err := applyExternalArtifactsDownload(i, &statefulset.Spec.Template.Spec); err != nil {
		ctx.RetryProcessing(err)
		return
	}

	addUserIdentities(ctx, i, statefulset)
	addUserConfigVolumes(ctx, i, statefulset)
	addTLS(ctx, i, statefulset)
	addXSiteTLS(ctx, i, statefulset)

	if err := ctx.SetControllerReference(statefulset); err != nil {
		ctx.RetryProcessing(err)
		return
	}
	ctx.Resources().Define(statefulset)
}

func addUserIdentities(ctx pipeline.Context, i *ispnv1.Infinispan, statefulset *appsv1.StatefulSet) {
	// Only append IDENTITIES_HASH and secret volume if authentication is enabled
	spec := &statefulset.Spec.Template.Spec
	ispnContainer := GetContainer(InfinispanContainer, spec)
	if AddVolumeForUserAuthentication(i, spec) {
		ispnContainer.Env = append(ispnContainer.Env,
			corev1.EnvVar{
				Name:  "IDENTITIES_HASH",
				Value: hash.HashByte(ctx.ConfigFiles().UserIdentities),
			})
	}
}

func addDataMountVolume(ctx pipeline.Context, i *ispnv1.Infinispan, statefulset *appsv1.StatefulSet) error {
	if i.IsEphemeralStorage() {
		volumes := &statefulset.Spec.Template.Spec.Volumes
		ephemeralVolume := corev1.Volume{
			Name: DataMountVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		*volumes = append(*volumes, ephemeralVolume)
		// TODO move warning to validating webhook
		errMsg := "Ephemeral storage configured. All data will be lost on cluster shutdown and restart."
		ctx.EventRecorder().Event(i, corev1.EventTypeWarning, EventReasonEphemeralStorage, errMsg)
		ctx.Log().Info(errMsg)
		return nil
	}

	var pvSize resource.Quantity
	if i.IsDataGrid() && i.StorageSize() != "" {
		pvSize, _ = resource.ParseQuantity(i.StorageSize())
	} else {
		_, memLimit, _ := i.Spec.Container.GetMemoryResources()
		if consts.DefaultPVSize.Cmp(memLimit) < 0 {
			pvSize = memLimit
		} else {
			pvSize = consts.DefaultPVSize
		}
	}

	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      DataMountVolume,
		Namespace: i.Namespace,
	},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: pvSize,
				},
			},
		},
	}
	if err := ctx.SetControllerReference(pvc); err != nil {
		return err
	}
	pvc.OwnerReferences[0].BlockOwnerDeletion = pointer.BoolPtr(false)
	// Set a storage class if it specified
	if storageClassName := i.StorageClassName(); storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}
	statefulset.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{*pvc}
	return nil
}

func addUserConfigVolumes(ctx pipeline.Context, i *ispnv1.Infinispan, statefulset *appsv1.StatefulSet) {
	if !i.UserConfigDefined() {
		return
	}
	userConfig := &ctx.ConfigFiles().UserConfig
	customConfig := userConfig.ServerConfig
	statefulset.Annotations["checksum/overlayConfig"] = hash.HashString(customConfig)

	volumes := &statefulset.Spec.Template.Spec.Volumes
	*volumes = append(*volumes, corev1.Volume{
		Name: UserConfVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: i.Spec.ConfigMapName},
			},
		}})

	container := GetContainer(InfinispanContainer, &statefulset.Spec.Template.Spec)
	volumeMounts := &container.VolumeMounts
	*volumeMounts = append(*volumeMounts, corev1.VolumeMount{
		Name:      UserConfVolumeName,
		MountPath: OverlayConfigMountPath,
	})
}

func buildStartupArgs(overlayConfigMapKey string, overlayLog4jConfig bool) []string {
	var args strings.Builder

	// Preallocate a buffer to speed up string building (saves code from growing the memory dynamically)
	args.Grow(110)

	// Check if the user defined a custom log4j config
	args.WriteString(" -l ")
	if overlayLog4jConfig {
		args.WriteString("user/log4j.xml")
	} else {
		args.WriteString(OperatorConfMountPath)
		args.WriteString("/log4j.xml")
	}

	// Check if the user defined an overlay operator config
	if overlayConfigMapKey != "" {
		args.WriteString(" -c user/")
		args.WriteString(overlayConfigMapKey)
	}
	args.WriteString(" -c operator/infinispan.xml")

	return strings.Fields(args.String())
}

func addTLS(ctx pipeline.Context, i *ispnv1.Infinispan, statefulset *appsv1.StatefulSet) {
	//applyExternalDependenciesVolume(i, &statefulset.Spec.Template.Spec)
	//if i.IsEncryptionEnabled() {
	//	AddVolumesForEncryption(i, &statefulset.Spec.Template.Spec)
	//	ispnContainer.Env = append(ispnContainer.Env,
	//		corev1.EnvVar{
	//			Name:  "KEYSTORE_HASH",
	//			Value: hash.HashMap(keystoreSecret.Data),
	//		})
	//
	//	if i.IsClientCertEnabled() {
	//		ispnContainer.Env = append(ispnContainer.Env,
	//			corev1.EnvVar{
	//				Name:  "TRUSTSTORE_HASH",
	//				Value: hash.HashMap(trustSecret.Data),
	//			})
	//	}
	//}
}

func addXSiteTLS(ctx pipeline.Context, i *ispnv1.Infinispan, statefulset *appsv1.StatefulSet) {

	//if i.IsSiteTLSEnabled() {
	//	AddSecretVolume(i.GetSiteTransportSecretName(), SiteTransportKeystoreVolumeName, consts.SiteTransportKeyStoreRoot, spec, InfinispanContainer)
	//	secret, err := FindSiteTrustStoreSecret(i, r.Client, r.ctx)
	//	if err != nil {
	//		return nil, err
	//	}
	//	if secret != nil {
	//		AddSecretVolume(i.GetSiteTrustoreSecretName(), SiteTruststoreVolumeName, consts.SiteTrustStoreRoot, spec, InfinispanContainer)
	//	}
	//}
}
