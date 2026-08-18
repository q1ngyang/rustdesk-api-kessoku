package service

import (
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	AllService = &Service{StarryControlService: NewStarryControlService(c, l)}
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
		offset := (page - 1) * pageSize
		return db.Offset(int(offset)).Limit(int(pageSize))
	}
}

func CommonEnable() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", model.COMMON_STATUS_ENABLE)
	}
}
