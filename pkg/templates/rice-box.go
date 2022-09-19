// Code generated by rice embed-go; DO NOT EDIT.
package templates

import (
	"time"

	"github.com/GeertJohan/go.rice/embedded"
)

func init() {

	// define files
	file4 := &embedded.EmbeddedFile{
		Filename:    "infinispan-admin-13.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<infinispan\n    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n    xsi:schemaLocation=\"urn:infinispan:config:13.0 https://infinispan.org/schemas/infinispan-config-13.0.xsd\n                        urn:infinispan:server:13.0 https://infinispan.org/schemas/infinispan-server-13.0.xsd\n                        urn:org:jgroups http://www.jgroups.org/schema/jgroups-4.2.xsd\"\n    xmlns=\"urn:infinispan:config:13.0\"\n    xmlns:server=\"urn:infinispan:server:13.0\">\n\n<jgroups>\n    <stack name=\"image-tcp\" extends=\"tcp\">\n        <TCP bind_addr=\"${jgroups.bind.address:SITE_LOCAL}\"\n             bind_port=\"${jgroups.bind.port,jgroups.tcp.port:7800}\"\n             enable_diagnostics=\"{{ .JGroups.Diagnostics }}\"\n             port_range=\"0\"\n        />\n        <dns.DNS_PING dns_query=\"{{ .StatefulSetName }}-ping.{{ .Namespace }}.svc.cluster.local\"\n                      dns_record_type=\"A\"\n                      stack.combine=\"REPLACE\" stack.position=\"MPING\"/>\n        {{ if .JGroups.FastMerge }}\n        <MERGE3 min_interval=\"1000\" max_interval=\"3000\" check_interval=\"5000\" stack.combine=\"COMBINE\"/>\n        {{ end }}\n    </stack>\n    {{ if .XSite }} {{ if .XSite.Sites }}\n    <stack name=\"relay-tunnel\" extends=\"udp\">\n        <TUNNEL\n            bind_addr=\"${jgroups.relay.bind.address:SITE_LOCAL}\"\n            bind_port=\"${jgroups.relay.bind.port:0}\"\n            gossip_router_hosts=\"{{RemoteSites .XSite.Sites}}\"\n            enable_diagnostics=\"{{ .JGroups.Diagnostics }}\"\n            port_range=\"0\"\n            {{ if .JGroups.FastMerge }}reconnect_interval=\"1000\"{{ end }}\n            stack.combine=\"REPLACE\"\n            stack.position=\"UDP\"\n        />\n        <!-- we are unable to use FD_SOCK with openshift -->\n        <!-- otherwise, we would need 1 external service per pod -->\n        <FD_SOCK stack.combine=\"REMOVE\"/>\n        {{ if .JGroups.FastMerge }}\n        <MERGE3 min_interval=\"1000\" max_interval=\"3000\" check_interval=\"5000\" stack.combine=\"COMBINE\"/>\n        {{ end }}\n    </stack>\n    <stack name=\"xsite\" extends=\"image-tcp\">\n        <relay.RELAY2 xmlns=\"urn:org:jgroups\" site=\"{{ (index .XSite.Sites 0).Name }}\" max_site_masters=\"{{ .XSite.MaxRelayNodes }}\" />\n        <remote-sites default-stack=\"relay-tunnel\">{{ range $it := .XSite.Sites }}\n            <remote-site name=\"{{ $it.Name }}\"/>\n        {{ end }}</remote-sites>\n    </stack>\n    {{ end }} {{ end }}\n</jgroups>\n<server xmlns=\"urn:infinispan:server:13.0\">\n    <interfaces>\n        <interface name=\"public\">\n            <inet-address value=\"${infinispan.bind.address}\"/>\n        </interface>\n    </interfaces>\n    <socket-bindings default-interface=\"public\" port-offset=\"${infinispan.socket.binding.port-offset:0}\">\n        <socket-binding name=\"admin\" port=\"11223\"/>\n    </socket-bindings>\n    <security>\n        <security-realms>\n            <security-realm name=\"admin\">\n                <properties-realm groups-attribute=\"Roles\">\n                    <user-properties path=\"cli-admin-users.properties\" relative-to=\"infinispan.server.config.path\"/>\n                    <group-properties path=\"cli-admin-groups.properties\" relative-to=\"infinispan.server.config.path\"/>\n                </properties-realm>\n            </security-realm>\n        </security-realms>\n    </security>\n    <endpoints>\n        <endpoint socket-binding=\"admin\" security-realm=\"admin\">\n            <rest-connector>\n                <authentication mechanisms=\"BASIC DIGEST\"/>\n            </rest-connector>\n            <hotrod-connector />\n        </endpoint>\n    </endpoints>\n</server>\n</infinispan>\n"),
	}
	file5 := &embedded.EmbeddedFile{
		Filename:    "infinispan-admin-14.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<infinispan\n    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n    xsi:schemaLocation=\"urn:infinispan:config:14.0 https://infinispan.org/schemas/infinispan-config-14.0.xsd\n                        urn:infinispan:server:14.0 https://infinispan.org/schemas/infinispan-server-14.0.xsd\n                        urn:org:jgroups http://www.jgroups.org/schema/jgroups-5.2.xsd\"\n    xmlns=\"urn:infinispan:config:14.0\"\n    xmlns:server=\"urn:infinispan:server:14.0\">\n\n<jgroups>\n    <stack name=\"image-tcp\" extends=\"tcp\">\n        <TCP bind_addr=\"${jgroups.bind.address:SITE_LOCAL}\"\n             bind_port=\"${jgroups.bind.port,jgroups.tcp.port:7800}\"\n             diag.enabled=\"{{ .JGroups.Diagnostics }}\"\n             port_range=\"0\"\n        />\n        <dns.DNS_PING dns_query=\"{{ .StatefulSetName }}-ping.{{ .Namespace }}.svc.cluster.local\"\n                      dns_record_type=\"A\"\n                      stack.combine=\"REPLACE\" stack.position=\"MPING\"/>\n        {{ if .JGroups.FastMerge }}\n        <MERGE3 min_interval=\"1000\" max_interval=\"3000\" check_interval=\"5000\" stack.combine=\"COMBINE\"/>\n        {{ end }}\n    </stack>\n    {{ if .XSite }} {{ if .XSite.Sites }}\n    <stack name=\"relay-tunnel\" extends=\"udp\">\n        <TUNNEL\n            bind_addr=\"${jgroups.relay.bind.address:SITE_LOCAL}\"\n            bind_port=\"${jgroups.relay.bind.port:0}\"\n            gossip_router_hosts=\"{{RemoteSites .XSite.Sites}}\"\n            diag.enabled=\"{{ .JGroups.Diagnostics }}\"\n            port_range=\"0\"\n            {{ if .JGroups.FastMerge }}reconnect_interval=\"1000\"{{ end }}\n            stack.combine=\"REPLACE\"\n            stack.position=\"UDP\"\n        />\n        <!-- we are unable to use FD_SOCK with openshift -->\n        <!-- otherwise, we would need 1 external service per pod -->\n        <FD_SOCK2 stack.combine=\"REMOVE\"/>\n        {{ if .JGroups.FastMerge }}\n        <MERGE3 min_interval=\"1000\" max_interval=\"3000\" check_interval=\"5000\" stack.combine=\"COMBINE\"/>\n        {{ end }}\n    </stack>\n    <stack name=\"xsite\" extends=\"image-tcp\">\n        <relay.RELAY2 xmlns=\"urn:org:jgroups\" site=\"{{ (index .XSite.Sites 0).Name }}\" max_site_masters=\"{{ .XSite.MaxRelayNodes }}\" />\n        <remote-sites default-stack=\"relay-tunnel\">{{ range $it := .XSite.Sites }}\n            <remote-site name=\"{{ $it.Name }}\"/>\n        {{ end }}</remote-sites>\n    </stack>\n    {{ end }} {{ end }}\n</jgroups>\n<server xmlns=\"urn:infinispan:server:14.0\">\n    <interfaces>\n        <interface name=\"public\">\n            <inet-address value=\"${infinispan.bind.address}\"/>\n        </interface>\n    </interfaces>\n    <socket-bindings default-interface=\"public\" port-offset=\"${infinispan.socket.binding.port-offset:0}\">\n        <socket-binding name=\"admin\" port=\"11223\"/>\n    </socket-bindings>\n    <security>\n        <security-realms>\n            <security-realm name=\"admin\">\n                <properties-realm groups-attribute=\"Roles\">\n                    <user-properties path=\"cli-admin-users.properties\" relative-to=\"infinispan.server.config.path\"/>\n                    <group-properties path=\"cli-admin-groups.properties\" relative-to=\"infinispan.server.config.path\"/>\n                </properties-realm>\n            </security-realm>\n        </security-realms>\n    </security>\n    <endpoints>\n        <endpoint socket-binding=\"admin\" security-realm=\"admin\">\n            <rest-connector>\n                <authentication mechanisms=\"BASIC DIGEST\"/>\n            </rest-connector>\n            <hotrod-connector />\n        </endpoint>\n    </endpoints>\n</server>\n</infinispan>\n"),
	}
	file6 := &embedded.EmbeddedFile{
		Filename:    "infinispan-base-13.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<infinispan\n    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n    xsi:schemaLocation=\"urn:infinispan:config:13.0 https://infinispan.org/schemas/infinispan-config-13.0.xsd\n                        urn:infinispan:server:13.0 https://infinispan.org/schemas/infinispan-server-13.0.xsd\n                        urn:infinispan:config:cloudevents:13.0 https://infinispan.org/schemas/infinispan-cloudevents-config-13.0.xsd\"\n    xmlns=\"urn:infinispan:config:13.0\"\n    xmlns:server=\"urn:infinispan:server:13.0\"\n    xmlns:ce=\"urn:infinispan:config:cloudevents:13.0\">\n\n<cache-container name=\"default\" statistics=\"true\">\n    {{ if .Infinispan.Authorization.Enabled }}\n    <security>\n        <authorization>\n            {{if eq .Infinispan.Authorization.RoleMapper \"commonName\" }}\n            <common-name-role-mapper />\n            {{ else }}\n            <cluster-role-mapper />\n            {{ end }}\n            {{ if .Infinispan.Authorization.Roles }}\n            {{ range $role :=  .Infinispan.Authorization.Roles }}\n            <role name=\"{{ $role.Name }}\" permissions=\"{{ $role.Permissions }}\"/>\n            {{ end }}\n            {{ end }}\n        </authorization>\n    </security>\n    {{ end }}\n    <transport cluster=\"${infinispan.cluster.name:{{ .ClusterName }}}\" node-name=\"${infinispan.node.name:}\"\n    {{if .XSite }}{{if .XSite.Sites }}stack=\"xsite\"{{ else }}stack=\"image-tcp\"{{ end }}{{ else }}stack=\"image-tcp\"{{ end }}\n    {{ if .Transport.TLS.Enabled }}server:security-realm=\"transport\"{{ end }}\n    />\n    {{ if .CloudEvents }}\n        <ce:cloudevents bootstrap-servers=\"{{ .CloudEvents.BootstrapServers }}\" {{if .CloudEvents.Acks }} acks=\"{{ .CloudEvents.Acks }}\" {{ end }} {{if .CloudEvents.CacheEntriesTopic }} cache-entries-topic=\"{{ .CloudEvents.CacheEntriesTopic }}\" {{ end }}/>\n    {{ end }}\n</cache-container>\n<server xmlns=\"urn:infinispan:server:13.0\">\n    <socket-bindings default-interface=\"public\" port-offset=\"${infinispan.socket.binding.port-offset:0}\">\n        <socket-binding name=\"default\" port=\"${infinispan.bind.port:11222}\"/>\n    </socket-bindings>\n    <security>\n        {{ if or .Keystore.Password .Truststore.Path }}\n        <credential-stores>\n          <credential-store name=\"credentials\" path=\"credentials.pfx\">\n            <clear-text-credential clear-text=\"secret\"/>\n          </credential-store>\n        </credential-stores>\n        {{ end }}\n        <security-realms>\n            <security-realm name=\"default\">\n                <server-identities>\n\t\t\t\t{{ if or .Keystore.Path .Truststore.Path}}\n\t\t\t\t<ssl>\n                        {{ if .Keystore.Path }}\n                            {{ if .Keystore.Password }}\n                                <keystore path=\"{{  .Keystore.Path }}\" {{if .Keystore.Alias }} alias=\"{{ .Keystore.Alias }}\" {{ end }}>\n                                    <credential-reference store=\"credentials\" alias=\"keystore\"/>\n                                </keystore>\n                            {{ else }}\n                                <keystore path=\"{{  .Keystore.Path }}\" keystore-password=\"\" {{if .Keystore.Alias }} alias=\"{{ .Keystore.Alias }}\" {{ end }}/>\n                            {{ end }}\n                        {{ end }}\n                        {{ if  .Truststore.Path }}\n                            <truststore path=\"{{ .Truststore.Path }}\">\n                                <credential-reference store=\"credentials\" alias=\"truststore\"/>\n                            </truststore>\n                        {{ end }}\n                </ssl>\n\t\t\t\t{{ end }}\n                </server-identities>\n                {{if .Endpoints.Authenticate }}\n                {{if eq .Endpoints.ClientCert \"Authenticate\" }}\n                <truststore-realm/>\n                {{ else }}\n                <properties-realm groups-attribute=\"Roles\">\n                    <user-properties path=\"cli-users.properties\" relative-to=\"infinispan.server.config.path\"/>\n                    <group-properties path=\"cli-groups.properties\" relative-to=\"infinispan.server.config.path\"/>\n                </properties-realm>\n                {{ end }}\n                {{ end }}\n            </security-realm>\n            {{ if .Transport.TLS.Enabled }}\n            <security-realm name=\"transport\">\n                <server-identities>\n                    <ssl>\n                        {{ if .Transport.TLS.KeyStore.Path }}\n                        <keystore path=\"{{ .Transport.TLS.KeyStore.Path }}\"\n                                    keystore-password=\"{{ .Transport.TLS.KeyStore.Password }}\"\n                                    alias=\"{{ .Transport.TLS.KeyStore.Alias }}\" />\n                        {{ end }}\n                        {{ if .Transport.TLS.TrustStore.Path }}\n                        <truststore path=\"{{ .Transport.TLS.TrustStore.Path }}\"\n                                    password=\"{{ .Transport.TLS.TrustStore.Password }}\" />\n                        {{ end }}\n                    </ssl>\n                </server-identities>\n            </security-realm>\n            {{ end }}\n        </security-realms>\n    </security>\n    <endpoints>\n        <endpoint socket-binding=\"default\" security-realm=\"default\" {{ if ne .Endpoints.ClientCert \"None\" }}require-ssl-client-auth=\"true\"{{ end }}>\n            {{ if .Endpoints.Authenticate }}\n            <hotrod-connector>\n                <authentication>\n                    <sasl qop=\"auth\" server-name=\"infinispan\"/>\n                </authentication>\n            </hotrod-connector>\n            {{ else }}\n            <hotrod-connector />\n            {{ end }}\n            <rest-connector />\n        </endpoint>\n    </endpoints>\n</server>\n</infinispan>\n"),
	}
	file7 := &embedded.EmbeddedFile{
		Filename:    "infinispan-base-14.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<infinispan\n    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n    xsi:schemaLocation=\"urn:infinispan:config:14.0 https://infinispan.org/schemas/infinispan-config-14.0.xsd\n                        urn:infinispan:server:14.0 https://infinispan.org/schemas/infinispan-server-14.0.xsd\n                        urn:infinispan:config:cloudevents:14.0 https://infinispan.org/schemas/infinispan-cloudevents-config-14.0.xsd\"\n    xmlns=\"urn:infinispan:config:14.0\"\n    xmlns:server=\"urn:infinispan:server:14.0\"\n    xmlns:ce=\"urn:infinispan:config:cloudevents:14.0\">\n\n<cache-container name=\"default\" statistics=\"true\">\n    {{ if .Infinispan.Authorization.Enabled }}\n    <security>\n        <authorization>\n            {{if eq .Infinispan.Authorization.RoleMapper \"commonName\" }}\n            <common-name-role-mapper />\n            {{ else }}\n            <cluster-role-mapper />\n            {{ end }}\n            {{ if .Infinispan.Authorization.Roles }}\n            {{ range $role :=  .Infinispan.Authorization.Roles }}\n            <role name=\"{{ $role.Name }}\" permissions=\"{{ $role.Permissions }}\"/>\n            {{ end }}\n            {{ end }}\n        </authorization>\n    </security>\n    {{ end }}\n    <transport cluster=\"${infinispan.cluster.name:{{ .ClusterName }}}\" node-name=\"${infinispan.node.name:}\"\n    {{if .XSite }}{{if .XSite.Sites }}stack=\"xsite\"{{ else }}stack=\"image-tcp\"{{ end }}{{ else }}stack=\"image-tcp\"{{ end }}\n    {{ if .Transport.TLS.Enabled }}server:security-realm=\"transport\"{{ end }}\n    />\n    {{ if .CloudEvents }}\n        <ce:cloudevents bootstrap-servers=\"{{ .CloudEvents.BootstrapServers }}\" {{if .CloudEvents.Acks }} acks=\"{{ .CloudEvents.Acks }}\" {{ end }} {{if .CloudEvents.CacheEntriesTopic }} cache-entries-topic=\"{{ .CloudEvents.CacheEntriesTopic }}\" {{ end }}/>\n    {{ end }}\n</cache-container>\n<server xmlns=\"urn:infinispan:server:14.0\">\n    <socket-bindings default-interface=\"public\" port-offset=\"${infinispan.socket.binding.port-offset:0}\">\n        <socket-binding name=\"default\" port=\"${infinispan.bind.port:11222}\"/>\n    </socket-bindings>\n    <security>\n        {{ if .FIPS }}\n        <providers>\n            {{ if .Keystore.Path }}\n<!--            <provider class-name=\"sun.security.pkcs11.SunPKCS11\" configuration=\"/tmp/server-keystore.cfg\"/>-->\n            {{ end }}\n\n            {{ if .Truststore.Path }}\n<!--            <provider class-name=\"sun.security.pkcs11.SunPKCS11\" configuration=\"/tmp/server-truststore.cfg\"/>-->\n            {{ end }}\n\n            {{ if and .Transport.TLS.Enabled }}\n            {{ if .Transport.TLS.KeyStore.Path}}\n<!--            <provider class-name=\"sun.security.pkcs11.SunPKCS11\" configuration=\"/tmp/transport-truststore.cfg\"/>-->\n            {{ end }}\n\n            {{ if .Transport.TLS.TrustStore.Path}}\n<!--            <provider class-name=\"sun.security.pkcs11.SunPKCS11\" configuration=\"/tmp/transport-truststore.cfg\"/>-->\n            {{ end }}\n            {{ end }}\n        </providers>\n        {{ else if or .Keystore.Password .Truststore.Path }}\n        <credential-stores>\n          <credential-store name=\"credentials\" path=\"credentials.pfx\">\n            <clear-text-credential clear-text=\"secret\"/>\n          </credential-store>\n        </credential-stores>\n        {{ end }}\n        <security-realms>\n            <security-realm name=\"default\">\n                <server-identities>\n\t\t\t\t{{ if or .Keystore.Path .Truststore.Path}}\n                <ssl>\n                        {{ if .FIPS }}\n                        <keystore provider=\"SunPKCS11-server-keystore\" type=\"PKCS11\"/>\n                        {{ else if .Keystore.Path }}\n                            {{ if .Keystore.Password }}\n                                <keystore path=\"{{  .Keystore.Path }}\" {{if .Keystore.Alias }} alias=\"{{ .Keystore.Alias }}\" {{ end }}>\n                                    <credential-reference store=\"credentials\" alias=\"keystore\"/>\n                                </keystore>\n                            {{ else }}\n                                <keystore path=\"{{  .Keystore.Path }}\" keystore-password=\"\" {{if .Keystore.Alias }} alias=\"{{ .Keystore.Alias }}\" {{ end }}/>\n                            {{ end }}\n                        {{ end }}\n\n                        {{ if .FIPS }}\n                            <truststore provider=\"SunPKCS11-server-truststore\" type=\"PKCS11\"/>\n                        {{ else if  .Truststore.Path }}\n                            <truststore path=\"{{ .Truststore.Path }}\">\n                                <credential-reference store=\"credentials\" alias=\"truststore\"/>\n                            </truststore>\n                        {{ end }}\n                </ssl>\n\t\t\t\t{{ end }}\n                </server-identities>\n                {{if .Endpoints.Authenticate }}\n                {{if eq .Endpoints.ClientCert \"Authenticate\" }}\n                <truststore-realm/>\n                {{ else }}\n                <properties-realm groups-attribute=\"Roles\">\n                    <user-properties path=\"cli-users.properties\" relative-to=\"infinispan.server.config.path\"/>\n                    <group-properties path=\"cli-groups.properties\" relative-to=\"infinispan.server.config.path\"/>\n                </properties-realm>\n                {{ end }}\n                {{ end }}\n            </security-realm>\n            {{ if .Transport.TLS.Enabled }}\n            <security-realm name=\"transport\">\n                <server-identities>\n                    <ssl>\n                        {{ if .FIPS }}\n                        <keystore provider=\"SunPKCS11-transport-keystore\" type=\"PKCS11\"/>\n                        {{ else if .Transport.TLS.KeyStore.Path }}\n                        <keystore path=\"{{ .Transport.TLS.KeyStore.Path }}\"\n                                    keystore-password=\"{{ .Transport.TLS.KeyStore.Password }}\"\n                                    alias=\"{{ .Transport.TLS.KeyStore.Alias }}\" />\n                        {{ end }}\n\n                        {{ if .FIPS }}\n                        <truststore provider=\"SunPKCS11-transport-truststore\" type=\"PKCS11\"/>\n                        {{ else if .Transport.TLS.TrustStore.Path }}\n                        <truststore path=\"{{ .Transport.TLS.TrustStore.Path }}\"\n                                    password=\"{{ .Transport.TLS.TrustStore.Password }}\" />\n                        {{ end }}\n                    </ssl>\n                </server-identities>\n            </security-realm>\n            {{ end }}\n        </security-realms>\n    </security>\n    <endpoints>\n        <endpoint socket-binding=\"default\" security-realm=\"default\" {{ if ne .Endpoints.ClientCert \"None\" }}require-ssl-client-auth=\"true\"{{ end }}>\n            {{ if .Endpoints.Authenticate }}\n            <hotrod-connector>\n                <authentication>\n                    <sasl qop=\"auth\" server-name=\"infinispan\"/>\n                </authentication>\n            </hotrod-connector>\n            {{ else }}\n            <hotrod-connector />\n            {{ end }}\n            <rest-connector />\n        </endpoint>\n    </endpoints>\n</server>\n</infinispan>\n"),
	}
	file8 := &embedded.EmbeddedFile{
		Filename:    "infinispan-zero-13.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<infinispan\n    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n    xsi:schemaLocation=\"urn:infinispan:config:13.0 https://infinispan.org/schemas/infinispan-config-13.0.xsd\n                        urn:infinispan:server:13.0 https://infinispan.org/schemas/infinispan-server-13.0.xsd\"\n    xmlns=\"urn:infinispan:config:13.0\"\n    xmlns:server=\"urn:infinispan:server:13.0\">\n\n<jgroups>\n    <stack name=\"image-tcp\" extends=\"tcp\">\n        <TCP bind_addr=\"${jgroups.bind.address:SITE_LOCAL}\"\n             bind_port=\"${jgroups.bind.port,jgroups.tcp.port:7800}\"\n             enable_diagnostics=\"{{ .JGroups.Diagnostics }}\"\n             port_range=\"0\"\n        />\n        <dns.DNS_PING dns_query=\"{{ .StatefulSetName }}-ping.{{ .Namespace }}.svc.cluster.local\"\n                      dns_record_type=\"A\"\n                      stack.combine=\"REPLACE\" stack.position=\"MPING\"/>\n        {{ if .JGroups.FastMerge }}\n        <MERGE3 min_interval=\"1000\" max_interval=\"3000\" check_interval=\"5000\" stack.combine=\"COMBINE\"/>\n        {{ end }}\n    </stack>\n</jgroups>\n<cache-container name=\"default\" statistics=\"true\" zero-capacity-node=\"true\">\n    {{ if .Infinispan.Authorization.Enabled }}\n    <security>\n        <authorization>\n            {{if eq .Infinispan.Authorization.RoleMapper \"commonName\" }}\n            <common-name-role-mapper />\n            {{ else }}\n            <cluster-role-mapper />\n            {{ end }}\n            {{ if .Infinispan.Authorization.Roles }}\n            {{ range $role :=  .Infinispan.Authorization.Roles }}\n            <role name=\"{{ $role.Name }}\" permissions=\"{{ $role.Permissions }}\"/>\n            {{ end }}\n            {{ end }}\n        </authorization>\n    </security>\n    {{ end }}\n    <transport cluster=\"${infinispan.cluster.name:{{ .ClusterName }}}\" node-name=\"${infinispan.node.name:}\"\n    stack=\"image-tcp\" />\n</cache-container>\n<server xmlns=\"urn:infinispan:server:13.0\">\n    <interfaces>\n        <interface name=\"public\">\n            <inet-address value=\"${infinispan.bind.address}\"/>\n        </interface>\n    </interfaces>\n    <socket-bindings default-interface=\"public\" port-offset=\"${infinispan.socket.binding.port-offset:0}\">\n        <socket-binding name=\"admin\" port=\"11223\"/>\n    </socket-bindings>\n    <security>\n        <security-realms>\n            <security-realm name=\"admin\">\n                <properties-realm groups-attribute=\"Roles\">\n                    <user-properties path=\"cli-admin-users.properties\" relative-to=\"infinispan.server.config.path\"/>\n                    <group-properties path=\"cli-admin-groups.properties\" relative-to=\"infinispan.server.config.path\"/>\n                </properties-realm>\n            </security-realm>\n        </security-realms>\n    </security>\n    <endpoints>\n        <endpoint socket-binding=\"admin\" security-realm=\"admin\">\n            <rest-connector>\n                <authentication mechanisms=\"BASIC DIGEST\"/>\n            </rest-connector>\n            <hotrod-connector />\n        </endpoint>\n    </endpoints>\n</server>\n</infinispan>\n"),
	}
	file9 := &embedded.EmbeddedFile{
		Filename:    "infinispan-zero-14.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<infinispan\n    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n    xsi:schemaLocation=\"urn:infinispan:config:14.0 https://infinispan.org/schemas/infinispan-config-14.0.xsd\n                        urn:infinispan:server:14.0 https://infinispan.org/schemas/infinispan-server-14.0.xsd\"\n    xmlns=\"urn:infinispan:config:14.0\"\n    xmlns:server=\"urn:infinispan:server:14.0\">\n\n<jgroups>\n    <stack name=\"image-tcp\" extends=\"tcp\">\n        <TCP bind_addr=\"${jgroups.bind.address:SITE_LOCAL}\"\n             bind_port=\"${jgroups.bind.port,jgroups.tcp.port:7800}\"\n             diag.enabled=\"{{ .JGroups.Diagnostics }}\"\n             port_range=\"0\"\n        />\n        <dns.DNS_PING dns_query=\"{{ .StatefulSetName }}-ping.{{ .Namespace }}.svc.cluster.local\"\n                      dns_record_type=\"A\"\n                      stack.combine=\"REPLACE\" stack.position=\"MPING\"/>\n        {{ if .JGroups.FastMerge }}\n        <MERGE3 min_interval=\"1000\" max_interval=\"3000\" check_interval=\"5000\" stack.combine=\"COMBINE\"/>\n        {{ end }}\n    </stack>\n</jgroups>\n<cache-container name=\"default\" statistics=\"true\" zero-capacity-node=\"true\">\n    {{ if .Infinispan.Authorization.Enabled }}\n    <security>\n        <authorization>\n            {{if eq .Infinispan.Authorization.RoleMapper \"commonName\" }}\n            <common-name-role-mapper />\n            {{ else }}\n            <cluster-role-mapper />\n            {{ end }}\n            {{ if .Infinispan.Authorization.Roles }}\n            {{ range $role :=  .Infinispan.Authorization.Roles }}\n            <role name=\"{{ $role.Name }}\" permissions=\"{{ $role.Permissions }}\"/>\n            {{ end }}\n            {{ end }}\n        </authorization>\n    </security>\n    {{ end }}\n    <transport cluster=\"${infinispan.cluster.name:{{ .ClusterName }}}\" node-name=\"${infinispan.node.name:}\"\n    stack=\"image-tcp\" />\n</cache-container>\n<server xmlns=\"urn:infinispan:server:14.0\">\n    <interfaces>\n        <interface name=\"public\">\n            <inet-address value=\"${infinispan.bind.address}\"/>\n        </interface>\n    </interfaces>\n    <socket-bindings default-interface=\"public\" port-offset=\"${infinispan.socket.binding.port-offset:0}\">\n        <socket-binding name=\"admin\" port=\"11223\"/>\n    </socket-bindings>\n    <security>\n        <security-realms>\n            <security-realm name=\"admin\">\n                <properties-realm groups-attribute=\"Roles\">\n                    <user-properties path=\"cli-admin-users.properties\" relative-to=\"infinispan.server.config.path\"/>\n                    <group-properties path=\"cli-admin-groups.properties\" relative-to=\"infinispan.server.config.path\"/>\n                </properties-realm>\n            </security-realm>\n        </security-realms>\n    </security>\n    <endpoints>\n        <endpoint socket-binding=\"admin\" security-realm=\"admin\">\n            <rest-connector>\n                <authentication mechanisms=\"BASIC DIGEST\"/>\n            </rest-connector>\n            <hotrod-connector />\n        </endpoint>\n    </endpoints>\n</server>\n</infinispan>\n"),
	}
	filea := &embedded.EmbeddedFile{
		Filename:    "log4j.xml",
		FileModTime: time.Unix(1620137619, 0),

		Content: string("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Configuration name=\"InfinispanServerConfig\" monitorInterval=\"60\" shutdownHook=\"disable\">\n    <Appenders>\n        <!-- Colored output on the console -->\n        <Console name=\"STDOUT\">\n            <PatternLayout pattern=\"%d{HH:mm:ss,SSS} %-5p (%t) [%c] %m%throwable%n\"/>\n        </Console>\n    </Appenders>\n\n    <Loggers>\n        <Root level=\"INFO\">\n            <AppenderRef ref=\"STDOUT\" level=\"TRACE\"/>\n        </Root>\n\n        {{- range $key, $value := .Categories }}\n        <Logger name=\"{{ $key }}\" level=\"{{ $value | UpperCase }}\"/>\n        {{- end }}\n    </Loggers>\n</Configuration>\n"),
	}

	// define dirs
	dir3 := &embedded.EmbeddedDir{
		Filename:   "",
		DirModTime: time.Unix(1620137619, 0),
		ChildFiles: []*embedded.EmbeddedFile{
			file4, // "infinispan-admin-13.xml"
			file5, // "infinispan-admin-14.xml"
			file6, // "infinispan-base-13.xml"
			file7, // "infinispan-base-14.xml"
			file8, // "infinispan-zero-13.xml"
			file9, // "infinispan-zero-14.xml"
			filea, // "log4j.xml"

		},
	}

	// link ChildDirs
	dir3.ChildDirs = []*embedded.EmbeddedDir{}

	// register embeddedBox
	embedded.RegisterEmbeddedBox(`templates`, &embedded.EmbeddedBox{
		Name: `templates`,
		Time: time.Unix(1620137619, 0),
		Dirs: map[string]*embedded.EmbeddedDir{
			"": dir3,
		},
		Files: map[string]*embedded.EmbeddedFile{
			"infinispan-admin-13.xml": file4,
			"infinispan-admin-14.xml": file5,
			"infinispan-base-13.xml":  file6,
			"infinispan-base-14.xml":  file7,
			"infinispan-zero-13.xml":  file8,
			"infinispan-zero-14.xml":  file9,
			"log4j.xml":               filea,
		},
	})
}
