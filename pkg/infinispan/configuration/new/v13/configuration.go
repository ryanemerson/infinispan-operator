package v13

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/infinispan/infinispan-operator/pkg/infinispan/configuration/new/api"
	"k8s.io/utils/pointer"
)

var schemaLocations = []string{
	"urn:infinispan:config:13.0 https://infinispan.org/schemas/infinispan-config-13.0.xsd",
	"urn:infinispan:server:13.0 https://infinispan.org/schemas/infinispan-server-13.0.xsd",
	"urn:infinispan:config:clustered-locks:13.0 https://infinispan.org/schemas/infinispan-clustered-locks-config-13.0.xsd",
	"urn:org:jgroups http://www.jgroups.org/schema/jgroups-4.2.xsd",
	"urn:infinispan:config:cloudevents:13.0 https://infinispan.org/schemas/infinispan-cloudevents-config-13.0.xsd",
}

var xmlns = Xmlns{
	Xmlns:             "urn:infinispan:config:13.0",
	XmlnsCe:           "urn:infinispan:config:cloudevents:13.0",
	XmlnsLocks:        "urn:infinispan:config:clustered-locks:13.0",
	XmlnsServer:       "urn:infinispan:server:13.0",
	XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
	XsiSchemaLocation: strings.Join(schemaLocations, " "),
}

type Infinispan struct {
	Xmlns
	XMLName        xml.Name `xml:"infinispan"`
	JGroups        *JGroups
	CacheContainer *CacheContainer
	Server         *Server
}

type Xmlns struct {
	Xmlns             string `xml:"xmlns,attr"`
	XmlnsCe           string `xml:"xmlns:ce,attr"`
	XmlnsLocks        string `xml:"xmlns:locks,attr"`
	XmlnsServer       string `xml:"xmlns:server,attr"`
	XmlnsXsi          string `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string `xml:"xsi:schemaLocation,attr"`
}

type CacheContainer struct {
	XMLName      xml.Name `xml:"cache-container"`
	Name         string   `xml:"name,attr"`
	Statistics   bool     `xml:"statistics,attr"`
	ZeroCapacity bool     `xml:"zero-capacity-node,attr"`
	CloudEvents  *CloudEvents
	Locks        *ClusteredLocks
	Security     *CacheContainerSecurity
	Transport    *CacheContainerTransport
}

type CacheContainerSecurity struct {
	XMLName       xml.Name `xml:"security"`
	Authorization *Authorization
}

type CacheContainerTransport struct {
	XMLName xml.Name `xml:"transport"`
	Cluster string   `xml:"cluster,attr"`
	// TODO remove?
	NodeName      string `xml:"node-name,attr"`
	Stack         string `xml:"stack,attr"`
	SecurityRealm string `xml:"server:security-realm,attr"`
}

type CloudEvents struct {
	XMLName           xml.Name `xml:"ce:cloudevents"`
	Acks              string   `xml:"acks,attr"`
	BootstrapServers  string   `xml:"bootstrap-servers,attr"`
	CacheEntriesTopic string   `xml:"cache-entries-topic,attr"`
}

// TODO can we remove this? Or just attributes that are always the defaults
type ClusteredLocks struct {
	XMLName     xml.Name `xml:"locks:clustered-locks"`
	NumOwners   int      `xml:"num-owners,attr"`
	Reliability string   `xml:"reliability,attr"`
}

type Authorization struct {
	XMLName              xml.Name `xml:"authorization"`
	CommonNameRoleMapper *CommonNameRoleMapper
	ClusterRoleMapper    *ClusterRoleMapper
	Roles                []AuthorizationRole
}

type CommonNameRoleMapper struct {
	XMLName xml.Name `xml:"common-name-role-mapper"`
}

type ClusterRoleMapper struct {
	XMLName xml.Name `xml:"cluster-role-mapper"`
}

type AuthorizationRole struct {
	XMLName     xml.Name `xml:"role"`
	Name        string   `xml:"name,attr"`
	Permissions string   `xml:"permissions,attr"`
}

type JGroups struct {
	XMLName xml.Name `xml:"jgroups"`
	Stacks  []JGroupsStack
}

type JGroupsStack struct {
	XMLName     xml.Name `xml:"stack"`
	Name        string   `xml:"name,attr"`
	Extends     string   `xml:"extends,attr"`
	RemoteSites *JGroupsRemoteSites
	DNSPing     *JGroupsDNSPing
	FDSock      *JGroupsFDSock
	Merge3      *JGroupsMerge3
	Relay2      *JGroupsRelay2
	TCP         *JGroupsTCP
	TCPPing     *JGroupsTCPPing
	Tunnel      *JGroupsTunnel
}

type JGroupsStackPosition struct {
	Combine  *string `xml:"stack.combine,attr"`
	Position *string `xml:"stack.position,attr"`
}

type JGroupsTCP struct {
	XMLName      xml.Name `xml:"TCP"`
	BindAddr     string   `xml:"bind_addr,attr"`
	BindPort     string   `xml:"bind_port,attr"`
	Diagnostics  bool     `xml:"enable_diagnostics,attr"`
	ExternalAddr *string  `xml:"external_addr,attr"`
	ExternalPort *int32   `xml:"external_port,attr"`
	PortRange    int      `xml:"port_range,attr"`
}

type JGroupsDNSPing struct {
	JGroupsStackPosition
	XMLName    xml.Name `xml:"dns.DNS_PING"`
	Query      string   `xml:"dns_query,attr"`
	RecordType string   `xml:"dns_record_type,attr"`
}

type JGroupsFDSock struct {
	JGroupsStackPosition
	XMLName xml.Name `xml:"FD_SOCK"`
}

type JGroupsMerge3 struct {
	JGroupsStackPosition
	XMLName       xml.Name `xml:"MERGE3"`
	MinInterval   int      `xml:"min_interval,attr"`
	MaxInterval   int      `xml:"max_interval,attr"`
	CheckInterval int      `xml:"check_interval,attr"`
}

type JGroupsRelay2 struct {
	XMLName            xml.Name `xml:"relay.RELAY2"`
	Site               string   `xml:"site,attr"`
	MaxSiteMasters     int      `xml:"max_site_masters,attr"`
	RelayNodeCandidate bool     `xml:"can_become_site_master,attr"`
}

type JGroupsRemoteSites struct {
	XMLName      xml.Name `xml:"remote-sites"`
	DefaultStack string   `xml:"default-stack,attr"`
	Sites        []JGroupsRemoteSite
}

type JGroupsRemoteSite struct {
	XMLName xml.Name `xml:"remote-site"`
	Name    string   `xml:"name,attr"`
}

type JGroupsTCPPing struct {
	JGroupsStackPosition
	XMLName      xml.Name `xml:"TCPPING"`
	InitialHosts string   `xml:"initial_hosts,attr"`
	PortRange    int      `xml:"port_range,attr"`
}

type JGroupsTunnel struct {
	JGroupsStackPosition
	JGroupsTCP
	XMLName           xml.Name `xml:"TUNNEL"`
	GossipRouterHosts string   `xml:"gossip_router_hosts,attr"`
}

type Server struct {
	Xmlns
	XMLName        xml.Name `xml:"server"`
	Interfaces     *ServerInterfaces
	SocketBindings *SocketBindings
	Security       *ServerSecurity
	Endpoints      *Endpoints
}

type ServerInterfaces struct {
	XMLName    xml.Name `xml:"interfaces"`
	Interfaces []ServerInterface
}

type ServerInterface struct {
	XMLName     xml.Name `xml:"interface"`
	Name        string   `xml:"name,attr"`
	InetAddress InetAddress
}

type InetAddress struct {
	XMLName xml.Name `xml:"inet-address"`
	Value   string   `xml:"value,attr"`
}

type SocketBindings struct {
	XMLName          xml.Name `xml:"socket-bindings"`
	DefaultInterface string   `xml:"default-interface,attr"`
	PortOffset       string   `xml:"port-offset,attr"`
	Bindings         []SocketBinding
}

type SocketBinding struct {
	XMLName xml.Name `xml:"socket-binding"`
	Name    string   `xml:"name,attr"`
	Port    string   `xml:"port,attr"`
}

type ServerSecurity struct {
	XMLName          xml.Name `xml:"security"`
	CredentialStores *CredentialStores
	SecurityRealms   *SecurityRealms
}

type CredentialStores struct {
	XMLName xml.Name `xml:"credential-stores"`
	Stores  []CredentialStore
}

type CredentialStore struct {
	XMLName             xml.Name `xml:"credential-store"`
	Name                string   `xml:"name,attr"`
	Path                string   `xml:"path,attr"`
	ClearTextCredential *ClearTextCredential
}

type ClearTextCredential struct {
	XMLName   xml.Name `xml:"clear-text-credentials"`
	ClearText string   `xml:"clear-text,attr"`
}

type SecurityRealms struct {
	XMLName xml.Name `xml:"security-realms"`
	Realms  []SecurityRealm
}

type SecurityRealm struct {
	XMLName          xml.Name `xml:"security-realm"`
	Name             string   `xml:"name,attr"`
	PropertiesRealm  *PropertiesRealm
	ServerIdentities *ServerIdentities
	TruststoreRealm  *TruststoreRealm
}

type ServerIdentities struct {
	XMLName xml.Name `xml:"server-identities"`
	SSL     *ServerIdentitySSL
}

type ServerIdentitySSL struct {
	XMLName    xml.Name `xml:"ssl"`
	Keystore   []Keystore
	Truststore []Truststore
}

type Keystore struct {
	XMLName                    xml.Name `xml:"keystore"`
	Alias                      string   `xml:"alias,attr"`
	Path                       string   `xml:"path,attr"`
	Password                   string   `xml:"password,attr"`
	GenerateSelfSignedCertHost string   `xml:"generate-self-signed-certificate-host,attr"`
	CredentialRef              *CredentialReference
}

type Truststore struct {
	XMLName       xml.Name `xml:"truststore"`
	Path          string   `xml:"path,attr"`
	CredentialRef *CredentialReference
}

type CredentialReference struct {
	XMLName xml.Name `xml:"credential-reference"`
	Alias   string   `xml:"alias,attr"`
	Store   string   `xml:"store,attr"`
}

type PropertiesRealm struct {
	XMLName         xml.Name `xml:"properties-realm"`
	GroupAttr       string   `xml:"group-attributes,attr"`
	UserProperties  *UserProperties
	GroupProperties *GroupProperties
}

type UserProperties struct {
	XMLName    xml.Name `xml:"user-properties"`
	Path       string   `xml:"path,attr"`
	RelativeTo string   `xml:"relative-to,attr"`
}

type GroupProperties struct {
	XMLName    xml.Name `xml:"group-properties"`
	Path       string   `xml:"path,attr"`
	RelativeTo string   `xml:"relative-to,attr"`
}

type TruststoreRealm struct {
	XMLName xml.Name `xml:"truststore-realm"`
}

type Endpoints struct {
	XMLName xml.Name `xml:"endpoints"`
}

type Endpoint struct {
	XMLName              xml.Name `xml:"endpoint"`
	RequireSSlClientAuth bool     `xml:"require-ssl-client-auth,attr"`
	SocketBinding        string   `xml:"socket-binding,attr"`
	SecurityRealm        string   `xml:"security-realm,attr"`
	HotRodConnector      *HotRodConnector
	RestConnector        *RestConnector
}

type HotRodConnector struct {
	XMLName        xml.Name `xml:"hotrod-connector"`
	Authentication *HotRodAuthentication
}

type HotRodAuthentication struct {
	XMLName xml.Name `xml:"authentication"`
	Sasl    *HotRodSasl
}

type HotRodSasl struct {
	XMLName    xml.Name `xml:"sasl"`
	QOP        string   `xml:"qop,attr"`
	ServerName string   `xml:"server-name,attr"`
}

type RestConnector struct {
	XMLName        xml.Name `xml:"rest-connector"`
	Authentication *RestAuthentication
}

type RestAuthentication struct {
	XMLName    xml.Name `xml:"authentication"`
	Mechanisms string   `xml:"mechanisms,attr"`
	CorsRules  *CorsRules
}

type CorsRules struct {
	XMLName xml.Name `xml:"cors-rules"`
	Rules   []CorsRule
}

type CorsRule struct {
	XMLName          xml.Name `xml:"cors-rule"`
	Name             string   `xml:"name,attr"`
	AllowCredentials bool     `xml:"allow-credentials,attr"`
	MaxAgeSeconds    int      `xml:"max-age-seconds,attr"`
}

type generator struct {
	// TODO
}

func (g *generator) Generate(c *api.Config) ([]byte, error) {
	// TODO

	defaultStackName, stacks := jgroupsStacks(c)
	i := &Infinispan{
		Xmlns: xmlns,
		JGroups: &JGroups{
			Stacks: stacks,
		},
		CacheContainer: &CacheContainer{
			Locks: &ClusteredLocks{}, // TODO remove?
			Transport: &CacheContainerTransport{
				Cluster: fmt.Sprintf("${infinispan.cluster.name:%s}", c.ClusterName),
				Stack:   defaultStackName,
			},
		},
		Server: &Server{
			Interfaces: &ServerInterfaces{
				Interfaces: []ServerInterface{
					{
						Name: "public",
						InetAddress: InetAddress{
							Value: "${infinispan.bind.address}",
						},
					},
				},
			},
			SocketBindings: &SocketBindings{
				DefaultInterface: "public",
				PortOffset:       "${infinispan.socket.binding.port-offset:0}",
				Bindings: []SocketBinding{
					{
						Name: "default",
						Port: "${infinispan.bind.port:11222}",
					},
					{
						Name: "admin",
						Port: "11223",
					},
				},
			},
			Security: &ServerSecurity{
				CredentialStores: &CredentialStores{
					Stores: []CredentialStore{
						{
							Name: "credentials",
							Path: "credentials.pfx",
							ClearTextCredential: &ClearTextCredential{
								ClearText: "secret",
							},
						},
					},
				},
				SecurityRealms: &SecurityRealms{
					Realms: securityRealms(c),
				},
			},
		},
	}
	return xml.MarshalIndent(i, "", "  ")
}

func securityRealms(c *api.Config) []SecurityRealm {
	defaultRealm := SecurityRealm{}

	adminRealm := SecurityRealm{
		Name: "admin",
		PropertiesRealm: &PropertiesRealm{
			UserProperties: &UserProperties{
				Path:       "cli-users.properties",
				RelativeTo: "infinispan.server.config.path",
			},
			GroupProperties: &GroupProperties{
				Path:       "cli-groups.properties",
				RelativeTo: "infinispan.server.config.path",
			},
		},
	}
	return []SecurityRealm{defaultRealm, adminRealm}
}

func jgroupsStacks(c *api.Config) (string, []JGroupsStack) {
	imageTcp := JGroupsStack{
		Name:    "image-tcp",
		Extends: "tcp",
		TCP: &JGroupsTCP{
			BindAddr:    "${jgroups.bind.address:SITE_LOCAL}",
			BindPort:    "${jgroups.bind.port,jgroups.tcp.port}: 7800",
			Diagnostics: c.JGroups.Diagnostics,
			PortRange:   0,
		},
		DNSPing: &JGroupsDNSPing{
			Query:      fmt.Sprintf("%s-ping.%s.svc.cluster.local", c.StatefulSetName, c.Namespace),
			RecordType: "A",
			JGroupsStackPosition: JGroupsStackPosition{
				Combine:  pointer.String("REPLACE"),
				Position: pointer.String("MPING"),
			},
		},
	}

	if c.JGroups.FastMerge {
		imageTcp.Merge3 = &JGroupsMerge3{
			MinInterval:   1000,
			MaxInterval:   3000,
			CheckInterval: 5000,
			JGroupsStackPosition: JGroupsStackPosition{
				Combine: pointer.String("COMBINE"),
			},
		}
	}
	// TODO handle if xsite.Sites is empty than just use imageTCP
	if c.Xsite == nil {
		return imageTcp.Name, []JGroupsStack{imageTcp}
	}
	return "xsite", []JGroupsStack{
		imageTcp,
		{
			Name:    "relay-tcp",
			Extends: "tcp",
			TCP: &JGroupsTCP{
				BindAddr:     "${jgroups.bind.address:SITE_LOCAL}",
				BindPort:     "${jgroups.bind.port,jgroups.tcp.port:0}",
				Diagnostics:  c.JGroups.Diagnostics,
				ExternalAddr: pointer.String("TODO"),
				ExternalPort: pointer.Int32(0), // TODO
				PortRange:    0,
			},
			TCPPing: &JGroupsTCPPing{
				InitialHosts: "TODO",
				PortRange:    0,
				JGroupsStackPosition: JGroupsStackPosition{
					Combine:  pointer.String("REPLACE"),
					Position: pointer.String("MPING"),
				},
			},
			FDSock: &JGroupsFDSock{
				JGroupsStackPosition: JGroupsStackPosition{
					Combine: pointer.String("REMOVE"),
				},
			},
			Merge3: &JGroupsMerge3{
				MinInterval:   1000,
				MaxInterval:   3000,
				CheckInterval: 5000,
				JGroupsStackPosition: JGroupsStackPosition{
					Combine: pointer.String("COMBINE"),
				},
			},
		},
		{
			Name:    "relay-tunnel",
			Extends: "udp",
			FDSock: &JGroupsFDSock{
				JGroupsStackPosition: JGroupsStackPosition{
					Combine: pointer.String("REMOVE"),
				},
			},
			Merge3: &JGroupsMerge3{
				MinInterval:   1000,
				MaxInterval:   3000,
				CheckInterval: 5000,
				JGroupsStackPosition: JGroupsStackPosition{
					Combine: pointer.String("COMBINE"),
				},
			},
			Tunnel: &JGroupsTunnel{
				JGroupsTCP: JGroupsTCP{
					BindAddr:    "${jgroups.bind.address:SITE_LOCAL}",
					BindPort:    "${jgroups.bind.port,jgroups.tcp.port:0}",
					Diagnostics: c.JGroups.Diagnostics,
					PortRange:   0,
				},
				GossipRouterHosts: "TODO",
			},
		},
		{
			Name:    "xsite",
			Extends: "image-tcp",
			Relay2: &JGroupsRelay2{
				Site:               "TODO",
				MaxSiteMasters:     1,
				RelayNodeCandidate: true,
			},
			RemoteSites: &JGroupsRemoteSites{
				DefaultStack: "relay-tcp",
				Sites: []JGroupsRemoteSite{
					{Name: "TODO"},
				},
			},
		},
	}
}
