package api

type ServerConfigurator interface {
	// Generate the config alone should be sufficient as we always move forward, i.e. generate config for version specific using input params
	// In the case of an update we use the latest version generator
	// For downgrades, we generate using the old generator impl
	Generate(*Config) ([]byte, error)
}

type Config struct {
	ClusterName     string
	Namespace       string
	StatefulSetName string
	JGroups         JGroups
	Keystore        *Keystore
	TrustStore      *Truststore
	Xsite           *XSite
}

type XSite struct {
	// TODO
}

type JGroups struct {
	Diagnostics bool
	FastMerge   bool
}

type Keystore struct {
	Path         string
	Password     string
	Alias        string
	CrtPath      string
	SelfSignCert string
	Type         string
}

type Truststore struct {
	Authenticate bool
	CaFile       string
	Certs        string
	Path         string
	Password     string
}
