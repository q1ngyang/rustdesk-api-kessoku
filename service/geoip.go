package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

const maxMMDBDownloadBytes int64 = 128 << 20

type GeoIPResult struct {
	IP         string `json:"ip"`
	Country    string `json:"country"`
	CountryISO string `json:"country_iso"`
	City       string `json:"city"`
	ASN        uint   `json:"asn"`
	ASNOrg     string `json:"asn_org"`
	Private    bool   `json:"private"`
	Available  bool   `json:"available"`
}

type geoCityRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}
type geoCountryRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}
type geoASNRecord struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

type GeoIPService struct {
	config    *config.Config
	directory string
	mu        sync.RWMutex
	city      *maxminddb.Reader
	country   *maxminddb.Reader
	asn       *maxminddb.Reader
	updating  atomic.Bool
}

func NewGeoIPService(cfg *config.Config) *GeoIPService {
	directory := filepath.Join(filepath.Dir(cfg.Media.Directory), "geoip")
	return &GeoIPService{config: cfg, directory: directory}
}

func (s *GeoIPService) Init() error {
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return fmt.Errorf("create GeoIP directory: %w", err)
	}
	if err := s.reloadReaders(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	go s.autoUpdateLoop()
	return nil
}

func (s *GeoIPService) Lookup(rawIP, language string) (GeoIPResult, error) {
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil {
		return GeoIPResult{}, errors.New("invalid IP address")
	}
	result := GeoIPResult{IP: ip.String(), Private: isNonPublicIP(ip)}
	if result.Private {
		result.Available = true
		return result, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cityReader, countryReader, asnReader := s.city, s.country, s.asn
	if cityReader == nil && countryReader == nil && asnReader == nil {
		return result, nil
	}
	locales := localePreference(language)
	if cityReader != nil {
		var record geoCityRecord
		if err := cityReader.Lookup(ip, &record); err != nil {
			return result, err
		}
		result.CountryISO = record.Country.ISOCode
		result.Country = localizedGeoName(record.Country.Names, locales)
		result.City = localizedGeoName(record.City.Names, locales)
	}
	// GeoLite2 City normally contains country data. Keep the smaller Country
	// database as an independent fallback so operators can still identify the
	// country while the City database is unavailable or has no matching row.
	if countryReader != nil && result.CountryISO == "" {
		var record geoCountryRecord
		if err := countryReader.Lookup(ip, &record); err != nil {
			return result, err
		}
		result.CountryISO = record.Country.ISOCode
		result.Country = localizedGeoName(record.Country.Names, locales)
	}
	if asnReader != nil {
		var record geoASNRecord
		if err := asnReader.Lookup(ip, &record); err != nil {
			return result, err
		}
		result.ASN, result.ASNOrg = record.Number, record.Org
	}
	result.Available = true
	return result, nil
}

func (s *GeoIPService) TriggerUpdate() bool {
	if !s.updating.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer s.updating.Store(false)
		if err := s.update(context.Background()); err != nil {
			Logger.WithError(err).Warn("update GeoIP databases")
			s.recordUpdate(nil, err)
		}
	}()
	return true
}

func (s *GeoIPService) IsUpdating() bool { return s.updating.Load() }

func (s *GeoIPService) autoUpdateLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		s.ensureFresh()
		<-ticker.C
	}
}

func (s *GeoIPService) ensureFresh() {
	setting, err := AllService.SystemSettingService.Get()
	if err != nil || !setting.GeoIPEnabled {
		return
	}
	interval := time.Duration(setting.GeoIPUpdateHours) * time.Hour
	if !s.databaseFilesPresent() || setting.GeoIPLastUpdatedAt == nil || time.Since(time.Unix(*setting.GeoIPLastUpdatedAt, 0)) >= interval {
		s.TriggerUpdate()
	}
}

func (s *GeoIPService) databaseFilesPresent() bool {
	for _, name := range []string{"GeoLite2-City.mmdb", "GeoLite2-Country.mmdb", "GeoLite2-ASN.mmdb"} {
		info, err := os.Stat(filepath.Join(s.directory, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return false
		}
	}
	return true
}

func (s *GeoIPService) update(ctx context.Context) error {
	setting, err := AllService.SystemSettingService.Get()
	if err != nil {
		return err
	}
	if !setting.GeoIPEnabled {
		return errors.New("GeoIP lookup is disabled")
	}
	client := safeMMDBHTTPClient()
	type pending struct{ temporary, destination string }
	files := []struct{ source, name string }{
		{setting.GeoIPCityURL, "GeoLite2-City.mmdb"},
		{setting.GeoIPCountryURL, "GeoLite2-Country.mmdb"},
		{setting.GeoIPASNURL, "GeoLite2-ASN.mmdb"},
	}
	pendingFiles := make([]pending, 0, len(files))
	defer func() {
		for _, file := range pendingFiles {
			_ = os.Remove(file.temporary)
		}
	}()
	for _, file := range files {
		temporary, err := s.downloadMMDB(ctx, client, file.source, file.name)
		if err != nil {
			return err
		}
		pendingFiles = append(pendingFiles, pending{temporary: temporary, destination: filepath.Join(s.directory, file.name)})
	}
	for _, file := range pendingFiles {
		if err := os.Rename(file.temporary, file.destination); err != nil {
			return err
		}
	}
	if err := s.reloadReaders(); err != nil {
		return err
	}
	now := time.Now().Unix()
	s.recordUpdate(&now, nil)
	return nil
}

func (s *GeoIPService) downloadMMDB(ctx context.Context, client *http.Client, source, name string) (string, error) {
	if err := validatePublicHTTPSURL(source); err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/octet-stream")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s returned HTTP %d", name, response.StatusCode)
	}
	if response.ContentLength > maxMMDBDownloadBytes {
		return "", fmt.Errorf("download %s exceeds size limit", name)
	}
	temporary, err := os.CreateTemp(s.directory, ".mmdb-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return "", err
	}
	written, err := io.Copy(temporary, io.LimitReader(response.Body, maxMMDBDownloadBytes+1))
	if err != nil {
		return "", err
	}
	if written > maxMMDBDownloadBytes {
		return "", fmt.Errorf("download %s exceeds size limit", name)
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	reader, err := maxminddb.Open(temporaryName)
	if err != nil {
		return "", fmt.Errorf("validate %s: %w", name, err)
	}
	_ = reader.Close()
	keep = true
	return temporaryName, nil
}

func (s *GeoIPService) reloadReaders() error {
	city, cityErr := maxminddb.Open(filepath.Join(s.directory, "GeoLite2-City.mmdb"))
	country, countryErr := maxminddb.Open(filepath.Join(s.directory, "GeoLite2-Country.mmdb"))
	asn, asnErr := maxminddb.Open(filepath.Join(s.directory, "GeoLite2-ASN.mmdb"))
	if cityErr != nil && !errors.Is(cityErr, os.ErrNotExist) {
		if country != nil {
			_ = country.Close()
		}
		if asn != nil {
			_ = asn.Close()
		}
		return cityErr
	}
	if countryErr != nil && !errors.Is(countryErr, os.ErrNotExist) {
		if city != nil {
			_ = city.Close()
		}
		if asn != nil {
			_ = asn.Close()
		}
		return countryErr
	}
	if asnErr != nil && !errors.Is(asnErr, os.ErrNotExist) {
		if city != nil {
			_ = city.Close()
		}
		if country != nil {
			_ = country.Close()
		}
		return asnErr
	}
	if city == nil && country == nil && asn == nil {
		return os.ErrNotExist
	}
	s.mu.Lock()
	oldCity, oldCountry, oldASN := s.city, s.country, s.asn
	s.city, s.country, s.asn = city, country, asn
	s.mu.Unlock()
	if oldCity != nil {
		_ = oldCity.Close()
	}
	if oldASN != nil {
		_ = oldASN.Close()
	}
	if oldCountry != nil {
		_ = oldCountry.Close()
	}
	return nil
}

func (s *GeoIPService) recordUpdate(timestamp *int64, updateErr error) {
	if DB == nil {
		return
	}
	setting := defaultSystemSetting()
	_ = DB.FirstOrCreate(setting, model.SystemSetting{IdModel: model.IdModel{Id: systemSettingSingletonID}}).Error
	values := map[string]interface{}{"geo_ip_last_error": ""}
	if timestamp != nil {
		values["geo_ip_last_updated_at"] = *timestamp
	}
	if updateErr != nil {
		values["geo_ip_last_error"] = truncateText(updateErr.Error(), 1000)
	}
	_ = DB.Model(setting).Updates(values).Error
}

func safeMMDBHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, candidate := range addresses {
			if isNonPublicIP(candidate.IP) {
				return nil, errors.New("MMDB endpoint resolved to a private or local address")
			}
		}
		if len(addresses) == 0 {
			return nil, errors.New("MMDB endpoint did not resolve")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second}
	return &http.Client{Transport: transport, Timeout: 2 * time.Minute, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("too many MMDB redirects")
		}
		return validatePublicHTTPSURL(request.URL.String())
	}}
}

func validatePublicHTTPSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("MMDB URL must be a valid public HTTPS URL")
	}
	return nil
}
func isNonPublicIP(ip net.IP) bool {
	return ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}
func localePreference(language string) []string {
	language = strings.ToLower(language)
	if strings.HasPrefix(language, "zh") {
		return []string{"zh-CN", "zh", "en"}
	}
	if strings.HasPrefix(language, "ja") {
		return []string{"ja", "en"}
	}
	return []string{"en"}
}
func localizedGeoName(names map[string]string, locales []string) string {
	for _, locale := range locales {
		if value := names[locale]; value != "" {
			return value
		}
	}
	for _, value := range names {
		return value
	}
	return ""
}
func truncateText(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}
