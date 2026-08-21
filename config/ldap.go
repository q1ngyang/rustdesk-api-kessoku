package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

var ldapAttributePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)

type LdapUser struct {
	BaseDn          string `mapstructure:"base-dn"`           // The base DN of the user for searching
	EnableAttr      string `mapstructure:"enable-attr"`       // The attribute name of the user for enabling, in AD it is "userAccountControl", empty means no enable attribute, all users are enabled
	EnableAttrValue string `mapstructure:"enable-attr-value"` // The value of the enable attribute when the user is enabled. If you are using AD, just leave it random str, it will be ignored.
	Filter          string `mapstructure:"filter"`
	Username        string `mapstructure:"username"`
	Email           string `mapstructure:"email"`
	FirstName       string `mapstructure:"first-name"`
	LastName        string `mapstructure:"last-name"`
	Sync            bool   `mapstructure:"sync"`        // Will sync the user's information to the internal database
	AdminGroup      string `mapstructure:"admin-group"` // Which group is the admin group
	AllowGroup      string `mapstructure:"allow-group"` // Which group is allowed to login
}

// type LdapGroup struct {
// 	BaseDn 		string            `mapstructure:"base-dn"` // The base DN of the group for searching
// 	Name        string            `mapstructure:"name"`    // The attribute name of the group
// 	Filter      string            `mapstructure:"filter"`
// 	Admin       string            `mapstructure:"admin"`   // Which group is the admin group
// 	Member      string            `mapstructure:"member"`  // How to get the member of the group: member, uniqueMember, or memberOf (default: member)
// 	Mode        string            `mapstructure:"mode"`
// 	Map         map[string]string `mapstructure:"map"`     // If mode is "map", map the LDAP group to the internal group
// }

type Ldap struct {
	Enable       bool     `mapstructure:"enable"`
	Url          string   `mapstructure:"url"`
	TlsCaFile    string   `mapstructure:"tls-ca-file"`
	TlsVerify    bool     `mapstructure:"tls-verify"`
	BaseDn       string   `mapstructure:"base-dn"`
	BindDn       string   `mapstructure:"bind-dn"`
	BindPassword string   `mapstructure:"bind-password"`
	User         LdapUser `mapstructure:"user"`
	// Group        LdapGroup `mapstructure:"group"`
}

func (l Ldap) Validate() error {
	if !l.Enable {
		return nil
	}
	parsed, err := url.Parse(l.Url)
	if err != nil {
		return fmt.Errorf("ldap.url: %w", err)
	}
	if parsed.Scheme != "ldaps" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("ldap.url must be an ldaps URL with a host and without credentials, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return errors.New("ldap.url path is not supported; configure ldap.base-dn instead")
	}
	if !l.TlsVerify {
		return errors.New("ldap.tls-verify must be true when LDAP is enabled")
	}
	if strings.TrimSpace(l.BaseDn) == "" || strings.TrimSpace(l.BindDn) == "" || l.BindPassword == "" {
		return errors.New("ldap.base-dn, ldap.bind-dn, and ldap.bind-password are required")
	}
	if _, err := ldap.ParseDN(l.BaseDn); err != nil {
		return fmt.Errorf("ldap.base-dn: %w", err)
	}
	for field, value := range map[string]string{
		"user.username":    l.User.Username,
		"user.email":       l.User.Email,
		"user.first-name":  l.User.FirstName,
		"user.last-name":   l.User.LastName,
		"user.enable-attr": l.User.EnableAttr,
	} {
		if value != "" && !ldapAttributePattern.MatchString(value) {
			return fmt.Errorf("ldap.%s must be a simple LDAP attribute name", field)
		}
	}
	if l.User.Filter != "" {
		if _, err := ldap.CompileFilter(l.User.Filter); err != nil {
			return fmt.Errorf("ldap.user.filter: %w", err)
		}
	}
	for field, dn := range map[string]string{
		"user.base-dn":     l.User.BaseDn,
		"user.admin-group": l.User.AdminGroup,
		"user.allow-group": l.User.AllowGroup,
	} {
		if dn != "" {
			if _, err := ldap.ParseDN(dn); err != nil {
				return fmt.Errorf("ldap.%s: %w", field, err)
			}
		}
	}
	if l.TlsCaFile != "" {
		info, err := os.Stat(l.TlsCaFile)
		if err != nil {
			return fmt.Errorf("ldap.tls-ca-file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return errors.New("ldap.tls-ca-file must be a regular file")
		}
	}
	return nil
}
