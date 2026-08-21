package config

import (
	"strings"
	"testing"
)

func validLDAPConfiguration() Ldap {
	return Ldap{
		Enable: true, Url: "ldaps://directory.example.test:636", TlsVerify: true,
		BaseDn: "dc=example,dc=test", BindDn: "cn=kessoku,dc=example,dc=test", BindPassword: "secret-from-runtime",
		User: LdapUser{Username: "uid", Email: "mail", Filter: "(objectClass=person)"},
	}
}

func TestLDAPValidationRequiresVerifiedEncryptedTransport(t *testing.T) {
	valid := validLDAPConfiguration()
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Ldap)
	}{
		{name: "plaintext", mutate: func(c *Ldap) { c.Url = "ldap://directory.example.test:389" }},
		{name: "no verification", mutate: func(c *Ldap) { c.TlsVerify = false }},
		{name: "filter syntax", mutate: func(c *Ldap) { c.User.Filter = "(&(objectClass=person)" }},
		{name: "attribute injection", mutate: func(c *Ldap) { c.User.Username = "uid)(|(uid=*" }},
		{name: "URL credentials", mutate: func(c *Ldap) { c.Url = "ldaps://user:pass@directory.example.test" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("unsafe LDAP configuration accepted")
			}
		})
	}
	if err := (Ldap{Enable: false, Url: strings.Repeat("invalid", 2)}).Validate(); err != nil {
		t.Fatalf("disabled LDAP configuration should be inert: %v", err)
	}
}
