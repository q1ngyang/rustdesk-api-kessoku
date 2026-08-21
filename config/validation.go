package config

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validGovernanceText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validProviderID(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && strings.ContainsRune("._-", character) {
			continue
		}
		return false
	}
	return true
}

func fixedHTTPSURL(value string, originOnly bool) (*url.URL, error) {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 2048 {
		return nil, errors.New("URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return nil, errors.New("URL must be an absolute HTTPS URL without credentials")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, errors.New("URL must not contain a query or fragment")
	}
	if originOnly && (parsed.Path != "" || parsed.RawPath != "") {
		return nil, errors.New("origin must not contain a path")
	}
	if !validURLHost(parsed) {
		return nil, errors.New("URL host is invalid")
	}
	return parsed, nil
}

func validURLHost(value *url.URL) bool {
	hostname := value.Hostname()
	if hostname == "" || strings.Contains(hostname, "%") {
		return false
	}
	if net.ParseIP(hostname) == nil && !validDNSName(hostname) {
		return false
	}
	return true
}

func canonicalOrigin(value *url.URL) string {
	hostname := strings.ToLower(value.Hostname())
	port := value.Port()
	if port == "443" && (value.Scheme == "https" || value.Scheme == "wss") {
		port = ""
	}
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	return strings.ToLower(value.Scheme) + "://" + host
}
