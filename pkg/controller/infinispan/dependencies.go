package infinispan

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strings"
	"text/template"

	infinispanv1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	kube "github.com/infinispan/infinispan-operator/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

const (
	CustomLibrariesMountPath               = "/opt/infinispan/server/lib/custom-libraries"
	CustomLibrariesVolumeName              = "custom-libraries"
	ExternalArtifactsMountPath             = "/opt/infinispan/server/lib/external-artifacts"
	ExternalArtifactsVolumeName            = "external-artifacts"
	ExternalArtifactsDownloadInitContainer = "external-artifacts-download"
	ExternalArtifactsTemporaryFileName     = "./tmp/$FILENAME"
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

func applyExternalArtifactsDownload(ispn *infinispanv1.Infinispan, spec *corev1.PodSpec) (updated bool, retErr error) {
	c := &spec.InitContainers
	volumes := &spec.Volumes
	volumeMounts := &spec.Containers[0].VolumeMounts
	containerPosition := kube.ContainerIndex(*c, ExternalArtifactsDownloadInitContainer)
	if ispn.HasExternalArtifacts() {
		extractCommands, err := externalArtifactsExtractCommand(ispn)
		if err != nil {
			retErr = err
			return
		}
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

func externalArtifactsExtractCommand(ispn *infinispanv1.Infinispan) (string, error) {
	templateStr := `set -e

	function retry {
		local n=1
		local max=5
		local delay=1
		while true; do
		  $@ && break || {
			if [[ $n -lt $max ]]; then
			  ((n++))
			  echo "Download failed. Attempt $n/$max:"
			  sleep $delay
			else
			  echo "Artifact download has failed after $n attempts."
			  exit 1
			fi
		  }
		done
	}

	cd {{ .MountPath }}
{{- range $i, $artifact := .Artifacts }}

	{{- if isType $artifact "zip" }}
	{{- $file := (printf "file%d.zip" $i) }}

	retry "curl --insecure -L {{ $artifact.Url }} -o {{ $file }}"
	{{ hashCmd $artifact $file }}
	unzip -oq {{ $file }}
	rm {{ $file }}

	{{- else if isType $artifact "tar.gz" }}
	{{- $file := (printf "file%d.zip" $i) }}

	retry "curl --insecure -L {{ $artifact.Url }} -o {{ $file }}"
	{{ hashCmd $artifact $file }}
	tar xf {{ $file }}
	rm {{ $file }}

	{{- else }}

	curl --insecure -LO {{ $artifact.Url }}
	{{ hashCmd $artifact (filename $artifact) }}
	{{- end }}

{{- end }}
	`

	getFilename := func(artifact infinispanv1.InfinispanExternalArtifacts) (string, error) {
		url, err := url.Parse(artifact.Url)
		if err != nil {
			return "", fmt.Errorf("Artifact url is not valid '%s'", artifact.Url)
		}
		return path.Base(url.Path), nil
	}

	tmpl, err := template.New("init-container").Funcs(template.FuncMap{
		"filename": getFilename,
		"isType": func(artifact infinispanv1.InfinispanExternalArtifacts, ext string) (bool, error) {
			if artifact.Type != "" {
				return string(artifact.Type) == ext, nil
			}
			fileName, err := getFilename(artifact)
			if err != nil {
				return false, err
			}
			return strings.HasSuffix(fileName, ext), nil
		},
		"hashCmd": func(artifact infinispanv1.InfinispanExternalArtifacts, localFile string) (string, error) {
			if artifact.Hash == "" {
				return "", nil
			}

			if !strings.Contains(artifact.Hash, ":") {
				return "", fmt.Errorf("Expected hash to be in the format `<hash-type>:<hash>`")
			}

			hashParts := strings.Split(artifact.Hash, ":")
			return fmt.Sprintf("echo %s %s | %ssum -c", hashParts[1], localFile, hashParts[0]), nil
		},
	}).Parse(templateStr)

	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	err = tmpl.Execute(&tpl, struct {
		MountPath string
		Artifacts []infinispanv1.InfinispanExternalArtifacts
	}{
		MountPath: ExternalArtifactsMountPath,
		Artifacts: ispn.Spec.Dependencies.Artifacts,
	})

	if err != nil {
		return "", err
	}
	return tpl.String(), nil
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
