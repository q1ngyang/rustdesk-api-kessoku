package api

import (
	"encoding/json"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
)

type AddressBookFormData struct {
	Tags      []string               `json:"tags"`
	Peers     []*AddressBookPeerForm `json:"peers"`
	TagColors string                 `json:"tag_colors"`
}

// AddressBookPeerForm is the client-sync allow list. Persistence identifiers,
// owners, collections, timestamps, and GORM associations are intentionally
// absent and therefore cannot be supplied by a client.
type AddressBookPeerForm struct {
	Id               string   `json:"id"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	Hostname         string   `json:"hostname"`
	Alias            string   `json:"alias"`
	Platform         string   `json:"platform"`
	Tags             []string `json:"tags"`
	Hash             string   `json:"hash"`
	ForceAlwaysRelay bool     `json:"forceAlwaysRelay"`
	RdpPort          string   `json:"rdpPort"`
	RdpUsername      string   `json:"rdpUsername"`
	Online           bool     `json:"online"`
	LoginName        string   `json:"loginName"`
	SameServer       bool     `json:"sameServer"`
}

func (f *AddressBookPeerForm) ToAddressBook() *model.AddressBook {
	tags, _ := json.Marshal(f.Tags)
	return &model.AddressBook{
		Id:               f.Id,
		Username:         f.Username,
		Password:         f.Password,
		Hostname:         f.Hostname,
		Alias:            f.Alias,
		Platform:         f.Platform,
		Tags:             tags,
		Hash:             f.Hash,
		ForceAlwaysRelay: f.ForceAlwaysRelay,
		RdpPort:          f.RdpPort,
		RdpUsername:      f.RdpUsername,
		Online:           f.Online,
		LoginName:        f.LoginName,
		SameServer:       f.SameServer,
	}
}

type AddressBookForm struct {
	Data string `json:"data" example:"{\"tags\":[\"tag1\",\"tag2\",\"tag3\"],\"peers\":[{\"id\":\"abc\",\"username\":\"abv-l\",\"hostname\":\"\",\"platform\":\"Windows\",\"alias\":\"\",\"tags\":[\"tag1\",\"tag2\"],\"hash\":\"hash\"}],\"tag_colors\":\"{\\\"tag1\\\":4288585374,\\\"tag2\\\":4278238420,\\\"tag3\\\":4291681337}\"}"`
}

type PeerForm struct {
	Cpu      string `json:"cpu"`
	Hostname string `json:"hostname"`
	Id       string `json:"id"`
	Memory   string `json:"memory"`
	Os       string `json:"os"`
	Username string `json:"username"`
	Uuid     string `json:"uuid"`
	Version  string `json:"version"`
}

func (pf *PeerForm) ToPeer() *model.Peer {
	return &model.Peer{
		Cpu:      pf.Cpu,
		Hostname: pf.Hostname,
		Id:       pf.Id,
		Memory:   pf.Memory,
		Os:       pf.Os,
		Username: pf.Username,
		Uuid:     pf.Uuid,
		Version:  pf.Version,
	}
}

// PersonalAddressBookForm 个人地址簿表单
type PersonalAddressBookForm struct {
	Id               string   `json:"id"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	Hostname         string   `json:"hostname"`
	Alias            string   `json:"alias"`
	Platform         string   `json:"platform"`
	Tags             []string `json:"tags"`
	Hash             string   `json:"hash"`
	RdpPort          string   `json:"rdpPort"`
	RdpUsername      string   `json:"rdpUsername"`
	Online           bool     `json:"online"`
	LoginName        string   `json:"loginName"`
	SameServer       bool     `json:"sameServer"`
	ForceAlwaysRelay string   `json:"forceAlwaysRelay"`
}

func (pabf *PersonalAddressBookForm) ToAddressBook() *model.AddressBook {
	tags, _ := json.Marshal(pabf.Tags)
	return &model.AddressBook{
		Id:               pabf.Id,
		Username:         pabf.Username,
		Password:         pabf.Password,
		Hostname:         pabf.Hostname,
		Alias:            pabf.Alias,
		Platform:         pabf.Platform,
		Tags:             tags,
		Hash:             pabf.Hash,
		ForceAlwaysRelay: pabf.ForceAlwaysRelay == "true",
		RdpPort:          pabf.RdpPort,
		RdpUsername:      pabf.RdpUsername,
		Online:           pabf.Online,
		LoginName:        pabf.LoginName,
		SameServer:       pabf.SameServer,
	}
}

type TagRenameForm struct {
	Old string `json:"old"`
	New string `json:"new"`
}
type TagColorForm struct {
	Name  string `json:"name"`
	Color uint   `json:"color"`
}

type PersonalAddressBookUpdate struct {
	Id       string    `json:"id" binding:"required,max=128"`
	Password *string   `json:"password"`
	Hash     *string   `json:"hash"`
	Tags     *[]string `json:"tags"`
	Alias    *string   `json:"alias"`
}

type PeerInfoInHeartbeat struct {
	Id   string `json:"id"`
	Uuid string `json:"uuid"`
	Ver  int    `json:"ver"`
}
