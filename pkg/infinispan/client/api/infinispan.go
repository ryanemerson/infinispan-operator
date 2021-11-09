package api

import (
	"bytes"

	"github.com/infinispan/infinispan-operator/pkg/mime"
)

type Infinispan interface {
	Cache(name string) Cache
	Caches() Caches
	Container() Container
	Cluster() Cluster
	Logging() Logging
	Metrics() Metrics
	Server() Server
}

type Container interface {
	Info() (*ContainerInfo, error)
	Backups() Backups
	Members() ([]string, error)
	Restores() Restores
	Xsite() Xsite
}

type Backups interface {
	Create(name string, config *BackupConfig) error
	Status(name string) (Status, error)
}

type Restores interface {
	Create(name string, config *RestoreConfig) error
	Status(name string) (Status, error)
}

type Cache interface {
	Config(contentType mime.MimeType) (string, error)
	Create(config string, contentType mime.MimeType) error
	CreateWithTemplate(templateName string) error
	Delete() error
	Exists() (bool, error)
	RollingUpgrade() RollingUpgrade
	UpdateConfig(config string, contentType mime.MimeType) error
}

type RollingUpgrade interface {
	AddSource(config string, contentType mime.MimeType) error
	DisconnectSource() error
	SourceConnected() (bool, error)
	SyncData() (string, error)
}

type Caches interface {
	ConvertConfiguration(config string, contentType, reqType mime.MimeType) (string, error)
	Names() ([]string, error)
}

type Cluster interface {
	GracefulShutdown() error
	GracefulShutdownTask() error
}

type Logging interface {
	GetLoggers() (map[string]string, error)
	SetLogger(name, level string) error
}

type Metrics interface {
	Get(postfix string) (buf *bytes.Buffer, err error)
}

type Server interface {
	Stop() error
}

type Xsite interface {
	PushAllState() error
}

type Status string

const (
	// StatusSucceeded means that the operation has completed.
	StatusSucceeded Status = "Succeeded"
	// StatusRunning means that the operation is in progress.
	StatusRunning Status = "Running"
	// StatusFailed means that the operation failed.
	StatusFailed Status = "Failed"
	// StatusUnknown means that the state of the operation could not be obtained, typically due to an error in communicating with the infinispan server.
	StatusUnknown Status = "Unknown"
)

type BackupConfig struct {
	Directory string `json:"directory" validate:"required"`
	// +optional
	Resources BackupRestoreResources `json:"resources,omitempty"`
}

type RestoreConfig struct {
	Location string `json:"location" validate:"required"`
	// +optional
	Resources BackupRestoreResources `json:"resources,omitempty"`
}

type BackupRestoreResources struct {
	// +optional
	Caches []string `json:"caches,omitempty"`
	// +optional
	Templates []string `json:"templates,omitempty"`
	// +optional
	Counters []string `json:"counters,omitempty"`
	// +optional
	ProtoSchemas []string `json:"proto-schemas,omitempty"`
	// +optional
	Tasks []string `json:"tasks,omitempty"`
}

type ContainerInfo struct {
	Coordinator bool           `json:"coordinator"`
	SitesView   *[]interface{} `json:"sites_view,omitempty"`
}
