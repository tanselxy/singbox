package panel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tanselxy/singbox/internal/model"
)

const clashSelfSignedSNI = "bing.com"

func clashSubscription(srv model.Server, nodes []namedServer, c model.Client) string {
	var proxies []clashProxy
	localLabel := ""
	if len(nodes) > 0 {
		localLabel = "本机"
	}
	proxies = append(proxies, clashProxies(srv, c, localLabel)...)
	for _, ns := range nodes {
		proxies = append(proxies, clashProxies(ns.Server, c, ns.Name)...)
	}

	var b strings.Builder
	b.WriteString("mixed-port: 7890\n")
	b.WriteString("allow-lan: false\n")
	b.WriteString("mode: rule\n")
	b.WriteString("log-level: info\n")
	b.WriteString("proxies:\n")
	if len(proxies) == 0 {
		b.WriteString("  []\n")
	} else {
		for _, proxy := range proxies {
			writeClashProxy(&b, proxy)
		}
	}
	b.WriteString("proxy-groups:\n")
	b.WriteString("  - name: " + yamlString("节点选择") + "\n")
	b.WriteString("    type: select\n")
	b.WriteString("    proxies:\n")
	if len(proxies) == 0 {
		b.WriteString("      - DIRECT\n")
	} else {
		for _, proxy := range proxies {
			b.WriteString("      - " + yamlString(proxy.Name) + "\n")
		}
		b.WriteString("      - DIRECT\n")
	}
	b.WriteString("rules:\n")
	b.WriteString("  - MATCH,节点选择\n")
	return b.String()
}

type clashProxy struct {
	Name   string
	Fields []clashField
}

type clashField struct {
	Key   string
	Value any
}

type clashMap []clashField
type clashList []string

func clashProxies(srv model.Server, c model.Client, label string) []clashProxy {
	display := c.Name
	if label != "" {
		display = label
	}
	if srv.IPv6Only {
		if srv.CDNDomain == "" {
			return nil
		}
		return []clashProxy{clashVLESSCDN(srv, c, display)}
	}
	proxies := []clashProxy{
		clashReality(srv, c, display),
		clashHysteria2(srv, c, display),
		clashTrojanWS(srv, c, display),
	}
	if srv.CDNDomain != "" {
		proxies = append(proxies, clashVLESSCDN(srv, c, display))
	}
	return proxies
}

func clashReality(srv model.Server, c model.Client, display string) clashProxy {
	return clashProxy{
		Name: display + "-Reality",
		Fields: []clashField{
			{"type", "vless"},
			{"server", srv.ServerIP},
			{"port", srv.Ports.Reality},
			{"uuid", c.UUID},
			{"network", "tcp"},
			{"tls", true},
			{"udp", true},
			{"flow", "xtls-rprx-vision"},
			{"servername", srv.SNI},
			{"client-fingerprint", "chrome"},
			{"reality-opts", clashMap{
				{"public-key", srv.Reality.PublicKey},
				{"short-id", srv.Reality.ShortID},
			}},
		},
	}
}

func clashHysteria2(srv model.Server, c model.Client, display string) clashProxy {
	return clashProxy{
		Name: display + "-Hysteria2",
		Fields: []clashField{
			{"type", "hysteria2"},
			{"server", srv.ServerIP},
			{"port", srv.Ports.Hysteria2},
			{"password", c.Password},
			{"sni", clashSelfSignedSNI},
			{"skip-cert-verify", true},
			{"alpn", clashList{"h3"}},
			{"udp", true},
		},
	}
}

func clashTrojanWS(srv model.Server, c model.Client, display string) clashProxy {
	return clashProxy{
		Name: display + "-Trojan",
		Fields: []clashField{
			{"type", "trojan"},
			{"server", srv.ServerIP},
			{"port", srv.Ports.TrojanWS},
			{"password", c.Password},
			{"network", "ws"},
			{"sni", clashSelfSignedSNI},
			{"skip-cert-verify", true},
			{"udp", true},
			{"ws-opts", clashMap{
				{"path", "/trojan"},
				{"headers", clashMap{{"Host", clashSelfSignedSNI}}},
			}},
		},
	}
}

func clashVLESSCDN(srv model.Server, c model.Client, display string) clashProxy {
	return clashProxy{
		Name: display + "-CDN",
		Fields: []clashField{
			{"type", "vless"},
			{"server", srv.CDNDomain},
			{"port", 443},
			{"uuid", c.UUID},
			{"network", "ws"},
			{"tls", true},
			{"udp", true},
			{"servername", srv.CDNDomain},
			{"ws-opts", clashMap{
				{"path", "/vless"},
				{"headers", clashMap{{"Host", srv.CDNDomain}}},
			}},
		},
	}
}

func writeClashProxy(b *strings.Builder, proxy clashProxy) {
	b.WriteString("  - name: " + yamlString(proxy.Name) + "\n")
	for _, field := range proxy.Fields {
		writeClashField(b, 4, field)
	}
}

func writeClashField(b *strings.Builder, indent int, field clashField) {
	pad := strings.Repeat(" ", indent)
	switch value := field.Value.(type) {
	case clashMap:
		b.WriteString(pad + field.Key + ":\n")
		for _, child := range value {
			writeClashField(b, indent+2, child)
		}
	case clashList:
		b.WriteString(pad + field.Key + ":\n")
		for _, item := range value {
			b.WriteString(strings.Repeat(" ", indent+2) + "- " + yamlString(item) + "\n")
		}
	default:
		b.WriteString(pad + field.Key + ": " + yamlScalar(value) + "\n")
	}
}

func yamlScalar(value any) string {
	switch v := value.(type) {
	case string:
		return yamlString(v)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return yamlString(fmt.Sprint(v))
	}
}

func yamlString(value string) string {
	return strconv.Quote(value)
}
