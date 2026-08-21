package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	// "golang.org/x/oauth2/google"
	"gorm.io/gorm"
	// "io"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OauthService struct {
}

// Define a struct to parse the .well-known/openid-configuration response
type OidcEndpoint struct {
	Issuer   string `json:"issuer"`
	AuthURL  string `json:"authorization_endpoint"`
	TokenURL string `json:"token_endpoint"`
	UserInfo string `json:"userinfo_endpoint"`
}

type OauthCacheItem struct {
	UserId          uint      `json:"-"`
	Id              string    `json:"id"` //rustdesk的设备ID
	Op              string    `json:"op"`
	Action          string    `json:"action"`
	Uuid            string    `json:"uuid"`
	DeviceName      string    `json:"device_name"`
	DeviceOs        string    `json:"device_os"`
	DeviceType      string    `json:"device_type"`
	OpenId          string    `json:"open_id"`
	Username        string    `json:"username"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	VerifiedEmail   bool      `json:"verified_email"`
	Verifier        string    `json:"-"` // used for oauth pkce
	Nonce           string    `json:"-"`
	CallbackClaimed bool      `json:"-"`
	ExpiresAt       time.Time `json:"-"`
}

func (oci *OauthCacheItem) ToOauthUser() *model.OauthUser {
	return &model.OauthUser{
		OpenId:        oci.OpenId,
		Username:      oci.Username,
		Name:          oci.Name,
		Email:         oci.Email,
		VerifiedEmail: oci.VerifiedEmail,
	}
}

const (
	maxOauthStates       = 4096
	oauthStateLifetime   = 5 * time.Minute
	maxOauthResponseBody = 1 << 20
)

type oauthStateStore struct {
	mu    sync.Mutex
	items map[string]*OauthCacheItem
}

var OauthCache = &oauthStateStore{items: make(map[string]*OauthCacheItem)}

const (
	OauthActionTypeLogin = "login"
	OauthActionTypeBind  = "bind"
)

func (oci *OauthCacheItem) UpdateFromOauthUser(oauthUser *model.OauthUser) {
	oci.OpenId = oauthUser.OpenId
	oci.Username = oauthUser.Username
	oci.Name = oauthUser.Name
	oci.Email = oauthUser.Email
	oci.VerifiedEmail = oauthUser.VerifiedEmail
}

func (os *OauthService) GetOauthCache(key string) *OauthCacheItem {
	OauthCache.mu.Lock()
	defer OauthCache.mu.Unlock()
	purgeExpiredOauthStatesLocked(time.Now())
	v, ok := OauthCache.items[key]
	if !ok {
		return nil
	}
	clone := *v
	return &clone
}

func (os *OauthService) SetOauthCache(key string, item *OauthCacheItem, expire uint) error {
	if key == "" || item == nil {
		return errors.New("invalid OAuth state")
	}
	OauthCache.mu.Lock()
	defer OauthCache.mu.Unlock()
	now := time.Now()
	purgeExpiredOauthStatesLocked(now)
	if _, exists := OauthCache.items[key]; !exists && len(OauthCache.items) >= maxOauthStates {
		return errors.New("OAuth state capacity reached")
	}
	clone := *item
	if expire > 0 || clone.ExpiresAt.IsZero() {
		lifetime := time.Duration(expire) * time.Second
		if lifetime <= 0 || lifetime > oauthStateLifetime {
			lifetime = oauthStateLifetime
		}
		clone.ExpiresAt = now.Add(lifetime)
	}
	OauthCache.items[key] = &clone
	return nil
}

func (os *OauthService) DeleteOauthCache(key string) {
	OauthCache.mu.Lock()
	delete(OauthCache.items, key)
	OauthCache.mu.Unlock()
}

func (os *OauthService) ClaimOauthCallback(key string) (*OauthCacheItem, bool) {
	OauthCache.mu.Lock()
	defer OauthCache.mu.Unlock()
	purgeExpiredOauthStatesLocked(time.Now())
	item, ok := OauthCache.items[key]
	if !ok || item == nil || item.CallbackClaimed || item.Op == "" || item.Action != OauthActionTypeLogin && item.Action != OauthActionTypeBind {
		return nil, false
	}
	item.CallbackClaimed = true
	clone := *item
	return &clone, true
}

func (os *OauthService) TakeOauthLoginResult(key, deviceID, deviceUUID string) (*OauthCacheItem, bool) {
	OauthCache.mu.Lock()
	defer OauthCache.mu.Unlock()
	purgeExpiredOauthStatesLocked(time.Now())
	item, ok := OauthCache.items[key]
	if !ok || item.Action != OauthActionTypeLogin || item.Id != deviceID || item.Uuid != deviceUUID {
		return nil, false
	}
	clone := *item
	if clone.UserId != 0 {
		delete(OauthCache.items, key)
	}
	return &clone, true
}

func purgeExpiredOauthStatesLocked(now time.Time) {
	for key, item := range OauthCache.items {
		if item == nil || !item.ExpiresAt.After(now) {
			delete(OauthCache.items, key)
		}
	}
}

func reserveOauthState(key string) error {
	return (&OauthService{}).SetOauthCache(key, &OauthCacheItem{}, uint(oauthStateLifetime/time.Second))
}

func (os *OauthService) BeginAuth(op string) (authErr error, state, verifier, nonce, authURL string) {
	externalOrigin, err := validateOauthExternalOrigin(Config.Rustdesk.ApiServer)
	if err != nil {
		return err, "", "", "", ""
	}
	state = utils.RandomString(32)
	if state == "" {
		return errors.New("generate OAuth state"), "", "", "", ""
	}
	if err := reserveOauthState(state); err != nil {
		return err, "", "", "", ""
	}
	defer func() {
		if authErr != nil {
			os.DeleteOauthCache(state)
		}
	}()
	verifier = ""
	nonce = ""
	if op == model.OauthTypeWebauth {
		authURL = externalOrigin + "/_admin/#/oauth/" + state
		//url = "http://localhost:8888/_admin/#/oauth/" + code
		return nil, state, verifier, nonce, authURL
	}
	err, oauthInfo, oauthConfig, _ := os.GetOauthConfig(op)
	if err == nil {
		extras := make([]oauth2.AuthCodeOption, 0, 3)

		nonce = utils.RandomString(32)
		if nonce == "" {
			return errors.New("generate OIDC nonce"), state, "", "", ""
		}
		extras = append(extras, oauth2.SetAuthURLParam("nonce", nonce))

		if oauthInfo.PkceEnable != nil && *oauthInfo.PkceEnable {
			extras = append(extras, oauth2.AccessTypeOffline)
			verifier = oauth2.GenerateVerifier()
			switch oauthInfo.PkceMethod {
			case model.PKCEMethodS256:
				extras = append(extras, oauth2.S256ChallengeOption(verifier))
			default:
				return errors.New("PKCE-enabled providers must use S256"), state, "", "", ""
			}
		}

		return err, state, verifier, nonce, oauthConfig.AuthCodeURL(state, extras...)
	}

	return err, state, verifier, nonce, ""
}

func (os *OauthService) FetchOidcProvider(issuer string) (error, *oidc.Provider) {
	if err := validateOauthEndpointURL(issuer); err != nil {
		return err, nil
	}

	// Get the HTTP client (with or without proxy based on configuration)
	client := getHTTPClientWithProxy()

	ctx := oidc.ClientContext(context.Background(), client)

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return err, nil
	}

	return nil, provider
}

func (os *OauthService) GithubProvider() *oidc.Provider {
	return (&oidc.ProviderConfig{
		IssuerURL:     "",
		AuthURL:       github.Endpoint.AuthURL,
		TokenURL:      github.Endpoint.TokenURL,
		DeviceAuthURL: github.Endpoint.DeviceAuthURL,
		UserInfoURL:   model.UserEndpointGithub,
		JWKSURL:       "",
		Algorithms:    nil,
	}).NewProvider(context.Background())
}

func (os *OauthService) LinuxdoProvider() *oidc.Provider {
	return (&oidc.ProviderConfig{
		IssuerURL:     "",
		AuthURL:       "https://connect.linux.do/oauth2/authorize",
		TokenURL:      "https://connect.linux.do/oauth2/token",
		DeviceAuthURL: "",
		UserInfoURL:   model.UserEndpointLinuxdo,
		JWKSURL:       "",
		Algorithms:    nil,
	}).NewProvider(context.Background())
}

// GetOauthConfig retrieves the OAuth2 configuration based on the provider name
func (os *OauthService) GetOauthConfig(op string) (err error, oauthInfo *model.Oauth, oauthConfig *oauth2.Config, provider *oidc.Provider) {
	//err, oauthInfo, oauthConfig = os.getOauthConfigGeneral(op)
	oauthInfo = os.InfoByOp(op)
	if oauthInfo.Id == 0 || oauthInfo.ClientId == "" || oauthInfo.ClientSecret == "" {
		return errors.New("ConfigNotFound"), nil, nil, nil
	}
	externalOrigin, err := validateOauthExternalOrigin(Config.Rustdesk.ApiServer)
	if err != nil {
		return err, nil, nil, nil
	}
	oauthConfig = &oauth2.Config{
		ClientID:     oauthInfo.ClientId,
		ClientSecret: oauthInfo.ClientSecret,
		RedirectURL:  externalOrigin + "/api/oidc/callback",
	}

	// Maybe should validate the oauthConfig here
	oauthType := oauthInfo.OauthType
	err = model.ValidateOauthType(oauthType)
	if err != nil {
		return err, nil, nil, nil
	}
	switch oauthType {
	case model.OauthTypeGithub:
		oauthConfig.Endpoint = github.Endpoint
		oauthConfig.Scopes = []string{"read:user", "user:email"}
		provider = os.GithubProvider()
	case model.OauthTypeLinuxdo:
		provider = os.LinuxdoProvider()
		oauthConfig.Endpoint = provider.Endpoint()
		oauthConfig.Scopes = []string{"profile"}
	//case model.OauthTypeGoogle: //google单独出来，可以少一次FetchOidcEndpoint请求
	//	oauthConfig.Endpoint = google.Endpoint
	//	oauthConfig.Scopes = os.constructScopes(oauthInfo.Scopes)
	case model.OauthTypeOidc, model.OauthTypeGoogle:
		err, provider = os.FetchOidcProvider(oauthInfo.Issuer)
		if err != nil {
			return err, nil, nil, nil
		}
		oauthConfig.Endpoint = provider.Endpoint()
		oauthConfig.Scopes = os.constructScopes(oauthInfo.Scopes)
	default:
		return errors.New("unsupported OAuth type"), nil, nil, nil
	}
	return nil, oauthInfo, oauthConfig, provider
}

func getHTTPClientWithProxy() *http.Client {
	timeout := 15 * time.Second
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, address := range addresses {
				if isDisallowedOauthIP(address.IP) {
					return nil, errors.New("OAuth endpoint resolved to a private or local address")
				}
			}
			if len(addresses) == 0 {
				return nil, errors.New("OAuth endpoint did not resolve")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	// OAuth/OIDC requests deliberately bypass application proxies. An HTTP
	// proxy resolves the target independently, so Kessoku could not prove that
	// the connected address is the public address validated above. Config
	// validation rejects proxy.enable instead of weakening this boundary.
	return &http.Client{Transport: boundedOauthTransport{base: transport}, Timeout: timeout, CheckRedirect: rejectOauthRedirect}
}

type boundedOauthTransport struct{ base http.RoundTripper }

func (t boundedOauthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := validateOauthEndpointURL(request.URL.String()); err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	response.Body = struct {
		io.Reader
		io.Closer
	}{Reader: io.LimitReader(response.Body, maxOauthResponseBody+1), Closer: response.Body}
	return response, nil
}

func rejectOauthRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func validateOauthEndpointURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return errors.New("OAuth endpoints must use an absolute HTTPS URL without credentials")
	}
	if strings.EqualFold(parsed.Hostname(), "localhost") || strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".localhost") {
		return errors.New("OAuth endpoint host is local")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && isDisallowedOauthIP(ip) {
		return errors.New("OAuth endpoint address is private or local")
	}
	return nil
}

func validateOauthExternalOrigin(raw string) (string, error) {
	if err := validateOauthEndpointURL(raw); err != nil {
		return "", fmt.Errorf("OAuth external origin: %w", err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("OAuth external origin must not include a path, query, or fragment")
	}
	return strings.TrimSuffix(raw, "/"), nil
}

func isDisallowedOauthIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

func (os *OauthService) callbackBase(oauthConfig *oauth2.Config, provider *oidc.Provider, code string, verifier string, nonce string, requireIDToken bool, userData interface{}) (err error, client *http.Client) {

	// 设置代理客户端
	httpClient := getHTTPClientWithProxy()
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	exchangeOpts := make([]oauth2.AuthCodeOption, 0, 1)
	if verifier != "" {
		exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(verifier))
	}

	token, err := oauthConfig.Exchange(ctx, code, exchangeOpts...)

	if err != nil {
		Logger.Warn("oauthConfig.Exchange() failed: ", err)
		return errors.New("GetOauthTokenError"), nil
	}

	// 获取 ID Token， github没有id_token
	rawIDToken, ok := token.Extra("id_token").(string)
	if requireIDToken && (!ok || rawIDToken == "") {
		return errors.New("IdTokenMissing"), nil
	}
	verifiedSubject := ""
	if ok && rawIDToken != "" {
		// 验证 ID Token
		v := provider.Verifier(&oidc.Config{ClientID: oauthConfig.ClientID})
		idToken, err2 := v.Verify(ctx, rawIDToken)
		if err2 != nil {
			Logger.Warn("IdTokenVerifyError: ", err2)
			return errors.New("IdTokenVerifyError"), nil
		}
		var claims struct {
			Nonce   string `json:"nonce"`
			Subject string `json:"sub"`
		}
		if err2 = idToken.Claims(&claims); err2 != nil {
			Logger.Warn("Failed to parse ID Token claims: ", err2)
			return errors.New("IDTokenClaimsError"), nil
		}
		if nonce != "" && claims.Nonce != nonce {
			Logger.Warn("Nonce does not match")
			return errors.New("NonceDoesNotMatch"), nil
		}
		if requireIDToken && strings.TrimSpace(claims.Subject) == "" {
			return errors.New("OIDCSubjectMissing"), nil
		}
		verifiedSubject = claims.Subject
	}

	// 获取用户信息
	client = oauthConfig.Client(ctx, token)
	resp, err := client.Get(provider.UserInfoEndpoint())
	if err != nil {
		Logger.Warn("failed getting user info: ", err)
		return errors.New("GetOauthUserInfoError"), nil
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			Logger.Warn("failed closing response body: ", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("GetOauthUserInfoError"), nil
	}

	// 解析用户信息
	if err = decodeBoundedOauthJSON(resp.Body, userData); err != nil {
		Logger.Warn("failed decoding user info: ", err)
		return errors.New("DecodeOauthUserInfoError"), nil
	}
	if requireIDToken {
		oidcUser, validType := userData.(*model.OidcUser)
		if !validType || !sameOidcSubject(verifiedSubject, oidcUser.Sub) {
			return errors.New("OIDCSubjectMismatch"), nil
		}
	}

	return nil, client
}

func decodeBoundedOauthJSON(reader io.Reader, destination interface{}) error {
	body, err := io.ReadAll(io.LimitReader(reader, maxOauthResponseBody+1))
	if err != nil {
		return err
	}
	if len(body) > maxOauthResponseBody {
		return errors.New("OAuth response exceeds size limit")
	}
	return json.Unmarshal(body, destination)
}

func sameOidcSubject(idTokenSubject, userInfoSubject string) bool {
	return strings.TrimSpace(idTokenSubject) != "" && idTokenSubject == userInfoSubject
}

// githubCallback github回调
func (os *OauthService) githubCallback(oauthConfig *oauth2.Config, provider *oidc.Provider, code, verifier, nonce string) (error, *model.OauthUser) {
	var user = &model.GithubUser{}
	err, client := os.callbackBase(oauthConfig, provider, code, verifier, nonce, false, user)
	if err != nil {
		return err, nil
	}
	err = os.getGithubPrimaryEmail(client, user)
	if err != nil {
		return err, nil
	}
	return nil, user.ToOauthUser()
}

// linuxdoCallback linux.do回调
func (os *OauthService) linuxdoCallback(oauthConfig *oauth2.Config, provider *oidc.Provider, code, verifier, nonce string) (error, *model.OauthUser) {
	var user = &model.LinuxdoUser{}
	err, _ := os.callbackBase(oauthConfig, provider, code, verifier, nonce, false, user)
	if err != nil {
		return err, nil
	}
	return nil, user.ToOauthUser()
}

// oidcCallback oidc回调, 通过code获取用户信息
func (os *OauthService) oidcCallback(oauthConfig *oauth2.Config, provider *oidc.Provider, code, verifier, nonce string) (error, *model.OauthUser) {
	var user = &model.OidcUser{}
	if err, _ := os.callbackBase(oauthConfig, provider, code, verifier, nonce, true, user); err != nil {
		return err, nil
	}
	return nil, user.ToOauthUser()
}

// Callback: Get user information by code and op(Oauth provider)
func (os *OauthService) Callback(code, verifier, op, nonce string) (err error, oauthUser *model.OauthUser) {
	err, oauthInfo, oauthConfig, provider := os.GetOauthConfig(op)
	// oauthType is already validated in GetOauthConfig
	if err != nil {
		return err, nil
	}
	oauthType := oauthInfo.OauthType
	switch oauthType {
	case model.OauthTypeGithub:
		err, oauthUser = os.githubCallback(oauthConfig, provider, code, verifier, nonce)
	case model.OauthTypeLinuxdo:
		err, oauthUser = os.linuxdoCallback(oauthConfig, provider, code, verifier, nonce)
	case model.OauthTypeOidc, model.OauthTypeGoogle:
		err, oauthUser = os.oidcCallback(oauthConfig, provider, code, verifier, nonce)
	default:
		return errors.New("unsupported OAuth type"), nil
	}
	if err == nil && (oauthUser == nil || strings.TrimSpace(oauthUser.OpenId) == "" || strings.TrimSpace(oauthUser.Username) == "") {
		return errors.New("OauthIdentityIncomplete"), nil
	}
	return err, oauthUser
}

func (os *OauthService) UserThirdInfo(op string, openId string) *model.UserThird {
	ut := &model.UserThird{}
	DB.Where("open_id = ? and op = ?", openId, op).First(ut)
	return ut
}

// BindOauthUser: Bind third party account
func (os *OauthService) BindOauthUser(userId uint, oauthUser *model.OauthUser, op string) error {
	if userId == 0 || oauthUser == nil || strings.TrimSpace(oauthUser.OpenId) == "" {
		return errors.New("invalid OAuth binding")
	}
	utr := &model.UserThird{}
	err, oauthType := os.GetTypeByOp(op)
	if err != nil {
		return err
	}
	utr.FromOauthUser(userId, oauthUser, oauthType, op)
	return DB.Create(utr).Error
}

// UnBindOauthUser: Unbind third party account
func (os *OauthService) UnBindOauthUser(userId uint, op string) error {
	return os.UnBindThird(op, userId)
}

// UnBindThird: Unbind third party account
func (os *OauthService) UnBindThird(op string, userId uint) error {
	return DB.Where("user_id = ? and op = ?", userId, op).Delete(&model.UserThird{}).Error
}

// DeleteUserByUserId: When user is deleted, delete all third party bindings
func (os *OauthService) DeleteUserByUserId(userId uint) error {
	return DB.Where("user_id = ?", userId).Delete(&model.UserThird{}).Error
}

// InfoById 根据id获取Oauth信息
func (os *OauthService) InfoById(id uint) *model.Oauth {
	oauthInfo := &model.Oauth{}
	DB.Where("id = ?", id).First(oauthInfo)
	return oauthInfo
}

// InfoByOp 根据op获取Oauth信息
func (os *OauthService) InfoByOp(op string) *model.Oauth {
	oauthInfo := &model.Oauth{}
	DB.Where("op = ?", op).First(oauthInfo)
	return oauthInfo
}

// Helper function to get scopes by operation
func (os *OauthService) getScopesByOp(op string) []string {
	scopes := os.InfoByOp(op).Scopes
	return os.constructScopes(scopes)
}

// Helper function to construct scopes
func (os *OauthService) constructScopes(scopes string) []string {
	scopes = strings.TrimSpace(scopes)
	if scopes == "" {
		scopes = model.OIDC_DEFAULT_SCOPES
	}
	return strings.Split(scopes, ",")
}

func (os *OauthService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.OauthList) {
	res = &model.OauthList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Oauth{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Oauths)
	return
}

// GetTypeByOp 根据op获取OauthType
func (os *OauthService) GetTypeByOp(op string) (error, string) {
	oauthInfo := &model.Oauth{}
	if DB.Where("op = ?", op).First(oauthInfo).Error != nil {
		return fmt.Errorf("OAuth provider with op '%s' not found", op), ""
	}
	return nil, oauthInfo.OauthType
}

// ValidateOauthProvider 验证Oauth提供者是否正确
func (os *OauthService) ValidateOauthProvider(op string) error {
	if !os.IsOauthProviderExist(op) {
		return fmt.Errorf("OAuth provider with op '%s' not found", op)
	}
	return nil
}

// IsOauthProviderExist 验证Oauth提供者是否存在
func (os *OauthService) IsOauthProviderExist(op string) bool {
	oauthInfo := &model.Oauth{}
	// 使用 Gorm 的 Take 方法查找符合条件的记录
	if err := DB.Where("op = ?", op).Take(oauthInfo).Error; err != nil {
		return false
	}
	return true
}

// Create 创建
func (os *OauthService) Create(oauthInfo *model.Oauth) error {
	err := oauthInfo.FormatOauthInfo()
	if err != nil {
		return err
	}
	res := DB.Create(oauthInfo).Error
	return res
}
func (os *OauthService) Delete(oauthInfo *model.Oauth) error {
	return DB.Delete(oauthInfo).Error
}

// Update 更新
func (os *OauthService) Update(oauthInfo *model.Oauth) error {
	err := oauthInfo.FormatOauthInfo()
	if err != nil {
		return err
	}
	return DB.Model(oauthInfo).Updates(oauthInfo).Error
}

// GetOauthProviders 获取所有的provider
func (os *OauthService) GetOauthProviders() []string {
	var res []string
	DB.Model(&model.Oauth{}).Pluck("op", &res)
	return res
}

// getGithubPrimaryEmail: Get the primary email of the user from Github
func (os *OauthService) getGithubPrimaryEmail(client *http.Client, githubUser *model.GithubUser) error {
	// the client is already set with the token
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return fmt.Errorf("failed to fetch emails: %w", err)
	}
	defer resp.Body.Close()

	// check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch emails: %s", resp.Status)
	}

	// decode the response
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := decodeBoundedOauthJSON(resp.Body, &emails); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// find the primary verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			githubUser.Email = e.Email
			githubUser.VerifiedEmail = e.Verified
			return nil
		}
	}

	return fmt.Errorf("no primary verified email found")
}
