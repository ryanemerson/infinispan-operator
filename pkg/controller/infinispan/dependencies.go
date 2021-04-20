package infinispan

import (
	"fmt"
	"path"
	"strings"

	infinispanv1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

const (
	CustomLibrariesMountPath                = "/opt/infinispan/server/lib/custom-libraries"
	CustomLibrariesVolumeName               = "custom-libraries"
	ExternalArtifactsMountPath              = "/opt/infinispan/server/lib/external-artifacts"
	ExternalArtifactsVolumeName             = "external-artifacts"
	ExternalArtifactsDownloadInitContainer  = "external-artifacts-download"
	ExternalArtifactsZipExtension           = ".zip"
	ExternalArtifactsTarGzExtension         = ".tar.gz"
	ExternalArtifactsArchiveDownloadCommand = "curl --insecure -L %s -o %s"
	ExternalArtifactsFileDownloadCommand    = "curl --insecure -LO %s"
	ExternalArtifactsDownloadRetryCommand   = "for i in 1 2 3 4 5; do %s && break || sleep 1; done"
	ExternalArtifactsZipExtractCommand      = "%s %s && unzip -oq %[3]s && rm %[3]s && "
	ExternalArtifactsTarGzExtractCommand    = "%s %s && tar xf %[3]s && rm %[3]s && "
	ExternalArtifactsFileExtractCommand     = "%s mkdir -p ./tmp && rm -rf ./tmp/* && cd ./tmp && %s && FILENAME=$(ls -1 . | head -n1) && cd .. %s && mv ./tmp/$FILENAME . && "
	ExternalArtifactsHashValidationCommand  = "&& echo %s %s | %ssum -c"
	ExternalArtifactsTemporaryFileName      = "./tmp/$FILENAME"
)

func applyExternalDependenciesVolume(ispn *infinispanv1.Infinispan, spec *corev1.PodSpec) (updated bool) {
	volumes := &spec.Volumes
	volumeMounts := &spec.Containers[0].VolumeMounts
	volumePosition := findVolume(*volumes, CustomLibrariesVolumeName)
	if ispn.HasDependenciesVolume() && volumePosition < 0 {
		*volumeMounts = append(*volumeMounts, corev1.VolumeMount{Name: CustomLibrariesVolumeName, MountPath: CustomLibrariesMountPath, ReadOnly: true})
		*volumes = append(*volumes, corev1.Volume{Name: CustomLibrariesVolumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: ispn.Spec.Dependencies.VolumeClaimName, ReadOnly: true}}})
		updated = true
	} else if !ispn.HasDependenciesVolume() && volumePosition >= 0 {
		volumeMountPosition := findVolumeMount(*volumeMounts, CustomLibrariesVolumeName)
		*volumes = append(spec.Volumes[:volumePosition], spec.Volumes[volumePosition+1:]...)
		*volumeMounts = append(spec.Containers[0].VolumeMounts[:volumeMountPosition], spec.Containers[0].VolumeMounts[volumeMountPosition+1:]...)
		updated = true
	}
	return
}

func applyExternalArtifactsDownload(ispn *infinispanv1.Infinispan, spec *corev1.PodSpec) (updated bool) {
	c := &spec.InitContainers
	volumes := &spec.Volumes
	volumeMounts := &spec.Containers[0].VolumeMounts
	containerPosition := kube.ContainerIndex(*c, ExternalArtifactsDownloadInitContainer)
	if ispn.HasExternalArtifacts() {
		extractCommands := externalArtifactsExtractCommand(ispn)
		if containerPosition >= 0 {
			if spec.InitContainers[containerPosition].Args[0] != extractCommands {
				spec.InitContainers[containerPosition].Args = []string{extractCommands}
				updated = true
			}
		} else {
			*c = append(*c, corev1.Container{
				Image:   ispn.ImageName(),
				Name:    ExternalArtifactsDownloadInitContainer,
				Command: []string{"sh", "-c"},
				Args:    []string{extractCommands},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      ExternalArtifactsVolumeName,
					MountPath: ExternalArtifactsMountPath,
				}},
			})
			*volumeMounts = append(*volumeMounts, corev1.VolumeMount{Name: ExternalArtifactsVolumeName, MountPath: ExternalArtifactsMountPath, ReadOnly: true})
			*volumes = append(*volumes, corev1.Volume{Name: ExternalArtifactsVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
			updated = true
		}
	} else if containerPosition >= 0 {
		volumePosition := findVolume(*volumes, ExternalArtifactsVolumeName)
		volumeMountPosition := findVolumeMount(*volumeMounts, ExternalArtifactsVolumeName)
		*c = append(spec.InitContainers[:containerPosition], spec.InitContainers[containerPosition+1:]...)
		*volumes = append(spec.Volumes[:volumePosition], spec.Volumes[volumePosition+1:]...)
		*volumeMounts = append(spec.Containers[0].VolumeMounts[:volumeMountPosition], spec.Containers[0].VolumeMounts[volumeMountPosition+1:]...)
		updated = true
	}
	return
}

func externalArtifactsExtractCommand(ispn *infinispanv1.Infinispan) string {
	var extractCommands = fmt.Sprintf("cd %s && ", ExternalArtifactsMountPath)
	for i, artifact := range ispn.Spec.Dependencies.Artifacts {
		fileName := strings.ToLower(path.Base(artifact.Url))
		if artifact.Type == infinispanv1.ExternalArtifactTypeZip || (artifact.Type == "" && strings.HasSuffix(fileName, ExternalArtifactsZipExtension)) {
			extractCommands = extractCommands + extractArchiveArtifactCommand(i, ExternalArtifactsZipExtension, artifact.Url, ExternalArtifactsZipExtractCommand, artifact.Hash)
			continue
		}
		if artifact.Type == infinispanv1.ExternalArtifactTypeTarGz || (artifact.Type == "" && strings.HasSuffix(fileName, ExternalArtifactsTarGzExtension)) {
			extractCommands = extractCommands + extractArchiveArtifactCommand(i, ExternalArtifactsTarGzExtension, artifact.Url, ExternalArtifactsTarGzExtractCommand, artifact.Hash)
			continue
		}
		downloadFileCommand := fmt.Sprintf(ExternalArtifactsDownloadRetryCommand, fmt.Sprintf(ExternalArtifactsFileDownloadCommand, artifact.Url))
		extractCommands = fmt.Sprintf(ExternalArtifactsFileExtractCommand, extractCommands, downloadFileCommand, hashValidationCommand(artifact.Hash, ExternalArtifactsTemporaryFileName))
	}
	return extractCommands + fmt.Sprintf("rm -rf %s/tmp", ExternalArtifactsMountPath)
}

func extractArchiveArtifactCommand(fileIndex int, fileExtension, downloadUrl, extractCommand, hash string) string {
	downloadFileName := fmt.Sprintf("file%d%s", fileIndex, fileExtension)
	downloadFileCommand := fmt.Sprintf(ExternalArtifactsDownloadRetryCommand, fmt.Sprintf(ExternalArtifactsArchiveDownloadCommand, downloadUrl, downloadFileName))
	return fmt.Sprintf(extractCommand, downloadFileCommand, hashValidationCommand(hash, downloadFileName), downloadFileName)
}

func hashValidationCommand(hash, fileName string) string {
	if hash == "" || !strings.Contains(hash, ":") {
		return ""
	}
	hashParts := strings.Split(hash, ":")
	return fmt.Sprintf(ExternalArtifactsHashValidationCommand, hashParts[1], fileName, hashParts[0])
}

func findVolume(volumes []corev1.Volume, volumeName string) int {
	for i, volume := range volumes {
		if volume.Name == volumeName {
			return i
		}
	}
	return -1
}

func findVolumeMount(volumeMounts []corev1.VolumeMount, volumeMountName string) int {
	for i, volumeMount := range volumeMounts {
		if volumeMount.Name == volumeMountName {
			return i
		}
	}
	return -1
}
