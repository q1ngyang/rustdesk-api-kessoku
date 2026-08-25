package api

import (
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

func TestValidAddressBookPeerRequiresBoundedStringTags(t *testing.T) {
	valid := &model.AddressBook{Id: "peer-1", Tags: []byte(`["one","two"]`)}
	if !validAddressBookPeer(valid) {
		t.Fatal("bounded string tags were rejected")
	}
	for _, tags := range [][]byte{
		[]byte(`{"not":"an array"}`),
		[]byte(`[1]`),
		[]byte(`[""]`),
	} {
		if validAddressBookPeer(&model.AddressBook{Id: "peer-1", Tags: tags}) {
			t.Fatalf("invalid tags were accepted: %s", tags)
		}
	}
}
