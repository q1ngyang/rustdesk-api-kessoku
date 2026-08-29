package service

import (
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	MaxPageSize  uint = 1000
	MaxBatchSize      = 1000
)

type Service struct {
	//AdminService     *AdminService
	//AdminRoleService *AdminRoleService
	*UserService
	*AddressBookService
	*TagService
	*PeerService
	*GroupService
	*OauthService
	*LoginLogService
	*AuditService
	*ShareRecordService
	*AuthIntrospectionService
	*StarryControlService
	*LdapService
	*AppService
	*AdminScopeService
	*BrandingService
	*TwoFactorService
	*SystemSettingService
	*GeoIPService
	*DataRetentionService
	NetworkPeerVerifier NetworkPeerVerifier
}

type Dependencies struct {
	Config *config.Config
	DB     *gorm.DB
	Logger *log.Logger
	Auth   *internalAuth.Manager
	Lock   *lock.Locker
}

var Config *config.Config
var DB *gorm.DB
var Logger *log.Logger
var Auth *internalAuth.Manager
var Lock lock.Locker

var AllService *Service

func New(c *config.Config, g *gorm.DB, l *log.Logger, authManager *internalAuth.Manager, lo lock.Locker) *Service {
	Config = c
	DB = g
	Logger = l
	Auth = authManager
	Lock = lo
	starryControl := NewStarryControlService(c, l, authManager)
	AllService = &Service{StarryControlService: starryControl, NetworkPeerVerifier: starryControl, BrandingService: &BrandingService{}, TwoFactorService: NewTwoFactorService(c.TwoFactor), SystemSettingService: &SystemSettingService{}, GeoIPService: NewGeoIPService(c), DataRetentionService: &DataRetentionService{}}
	return AllService
}

func Paginate(page, pageSize uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page == 0 {
			page = 1
		}
		if pageSize == 0 {
			pageSize = 10
		}
		if pageSize > MaxPageSize {
			pageSize = MaxPageSize
		}
		pageIndex := uint64(page - 1)
		size := uint64(pageSize)
		maximumInt := uint64(^uint(0) >> 1)
		if pageIndex > maximumInt/size {
			return db.Where("1 = 0").Limit(int(pageSize))
		}
		return db.Offset(int(pageIndex * size)).Limit(int(pageSize))
	}
}

func CommonEnable() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", model.COMMON_STATUS_ENABLE)
	}
}
