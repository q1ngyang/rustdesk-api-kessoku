package service

import (
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

func TestAddressBookSyncDoesNotCrossTenantPeerMetadataOrTrustRowIDs(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.Peer{}, &model.AddressBook{}, &model.Tag{}); err != nil {
		t.Fatal(err)
	}
	otherPeer := &model.Peer{Id: "other-peer", UserId: 2, Username: "other-secret", Hostname: "other-host", Os: "Linux"}
	ownedPeer := &model.Peer{Id: "owned-peer", UserId: 1, Username: "owned-user", Hostname: "owned-host", Os: "Windows"}
	if err := database.Create(otherPeer).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(ownedPeer).Error; err != nil {
		t.Fatal(err)
	}

	input := []*model.AddressBook{
		{RowId: 9001, Id: otherPeer.Id, CollectionId: 42, Collection: &model.AddressBookCollection{UserId: 999, Name: "untrusted"}},
		{RowId: 9002, Id: ownedPeer.Id, CollectionId: 42},
	}
	if err := AllService.AddressBookService.UpdateAddressBook(input, 1); err != nil {
		t.Fatal(err)
	}
	otherAddress := AllService.AddressBookService.InfoByUserIdAndIdAndCid(1, otherPeer.Id, 0)
	if otherAddress.RowId == 0 || otherAddress.RowId == 9001 || otherAddress.Username != "" || otherAddress.Hostname != "" {
		t.Fatalf("cross-tenant peer metadata or caller row id was trusted: %+v", otherAddress)
	}
	ownedAddress := AllService.AddressBookService.InfoByUserIdAndIdAndCid(1, ownedPeer.Id, 0)
	if ownedAddress.RowId == 0 || ownedAddress.RowId == 9002 || ownedAddress.Username != ownedPeer.Username || ownedAddress.Hostname != ownedPeer.Hostname {
		t.Fatalf("owned peer enrichment failed: %+v", ownedAddress)
	}
	var collectionCount int64
	if err := database.Model(&model.AddressBookCollection{}).Where("name = ?", "untrusted").Count(&collectionCount).Error; err != nil {
		t.Fatal(err)
	}
	if collectionCount != 0 {
		t.Fatal("nested address-book association was persisted")
	}

	tag := &model.Tag{Name: "safe", UserId: 1, Collection: &model.AddressBookCollection{UserId: 999, Name: "nested-tag-collection"}}
	if err := AllService.TagService.Create(tag); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.AddressBookCollection{}).Where("name = ?", "nested-tag-collection").Count(&collectionCount).Error; err != nil {
		t.Fatal(err)
	}
	if collectionCount != 0 || tag.Collection != nil {
		t.Fatal("nested tag association was persisted")
	}
}

func TestAddressBookAndTagsSyncIsAtomic(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.Peer{}, &model.AddressBook{}, &model.Tag{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`CREATE TRIGGER reject_tag_insert BEFORE INSERT ON tags BEGIN SELECT RAISE(FAIL, 'tag insert rejected'); END`).Error; err != nil {
		t.Fatal(err)
	}
	err := AllService.AddressBookService.UpdateAddressBookAndTags(
		[]*model.AddressBook{{Id: "peer-1", Tags: []byte(`[]`)}},
		1,
		map[string]uint{"tag-1": 1},
	)
	if err == nil {
		t.Fatal("tag failure was accepted")
	}
	var addressCount int64
	if err := database.Model(&model.AddressBook{}).Count(&addressCount).Error; err != nil {
		t.Fatal(err)
	}
	if addressCount != 0 {
		t.Fatal("address-book transaction committed after tag update failed")
	}
}
