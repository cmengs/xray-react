package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// NormalizeProtocol maps various incoming names to canonical protocol names.
func NormalizeProtocol(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	switch {
	case s == "ws" || strings.Contains(s, "websocket"):
		return "websocket"
	case s == "vmess":
		return "vmess"
	case s == "vless":
		return "vless"
	case s == "trojan":
		return "trojan"
	case s == "shadowsocks" || s == "ss" || s == "less":
		return "shadowsocks"
	case s == "http":
		return "http"
	default:
		return s
	}
}

// ExtractMeta canonicalizes and extracts common protocol-specific handshake fields
// from an incoming meta map. It's tolerant: if a field isn't present, it won't add it.
func ExtractMeta(in map[string]interface{}, protocol string) map[string]interface{} {
	if in == nil {
		in = map[string]interface{}{}
	}

	out := map[string]interface{}{}

	// helper to copy if string
	copyIfString := func(keys ...string) (string, bool) {
		for _, k := range keys {
			if v, ok := in[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s, true
				}
				// sometimes numbers or other types: format to string
				return fmt.Sprintf("%v", v), true
			}
		}
		return "", false
	}

	// Common fields
	if sni, ok := copyIfString("sni", "host", "server_name"); ok {
		out["sni"] = sni
	}
	if user, ok := copyIfString("user", "username", "account"); ok {
		out["user"] = user
	}

	// Protocol-specific extraction (common fields)
	switch protocol {
	case "websocket":
		// ws path and subprotocol are commonly present
		if p, ok := copyIfString("ws_path", "path", "uri"); ok {
			out["ws_path"] = p
		}
		if sp, ok := copyIfString("ws_subprotocol", "subprotocol", "sub_protocol"); ok {
			out["ws_subprotocol"] = sp
		}
	case "vmess", "vless":
		// vmess/vless usually have id/uuid and maybe alterId or aid
		if id, ok := copyIfString("id", "uuid", "user_id"); ok {
			out["id"] = id
		}
		// attempt to extract peer or remark fields
		if remark, ok := copyIfString("remark", "tag", "remark_name"); ok {
			out["remark"] = remark
		}
	case "trojan":
		// trojan uses password as the user credential; some agents may call it "password" or "trojan_user"
		if u, ok := copyIfString("user", "trojan_user", "password", "pass"); ok {
			out["user"] = u
		}
		// trojan may include sni as well
		if sni, ok := copyIfString("sni", "host"); ok {
			out["sni"] = sni
		}
	case "shadowsocks":
		// ss may include method and cipher, user rarely present
		if method, ok := copyIfString("method", "cipher"); ok {
			out["method"] = method
		}
		if plugin, ok := copyIfString("plugin", "plugin_opts"); ok {
			out["plugin"] = plugin
		}
	}

	// Try to parse incoming "remote" or "target" if present to canonical dst info
	if target, ok := copyIfString("target", "destination", "remote_addr"); ok {
		// simple parse ip:port
		re := regexp.MustCompile(`^(.+?):(\d+)$`)
		if m := re.FindStringSubmatch(target); len(m) == 3 {
			out["dst_addr"] = m[1]
			out["dst_port"] = m[2]
		} else {
			out["dst"] = target
		}
	}

	// copy any unknown keys with small whitelist to avoid huge meta
	whitelist := []string{"network", "tls", "tls_version", "cipher", "sni", "path", "host", "remark"}
	for _, k := range whitelist {
		if v, ok := in[k]; ok {
			out[k] = v
		}
	}

	// keep original meta keys for debugging under "raw_meta" if present
	// but avoid copying huge objects; only keep small map if not empty
	if len(in) > 0 {
		out["raw_meta"] = in
	}
	return out
}
