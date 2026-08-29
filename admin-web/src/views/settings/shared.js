import { systemSettings, updateSystemSettings } from '@/api/config'

export const systemSettingDefaults = Object.freeze({
  announcement: '',
  geoip_enabled: true,
  geoip_city_url: 'https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb',
  geoip_country_url: 'https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb',
  geoip_asn_url: 'https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb',
  geoip_update_hours: 168,
  geoip_last_updated_at: null,
  geoip_last_error: '',
  geoip_updating: false,
  web_login_hours: 168,
  client_login_hours: 168,
  login_maximum_hours: 168,
  user_token_retention_days: 0,
  login_log_retention_days: 0,
  audit_conn_retention_days: 0,
  audit_file_retention_days: 0,
  control_audit_retention_days: 0,
})

export const loadSystemSettingForm = async form => {
  Object.assign(form, systemSettingDefaults, (await systemSettings()).data || {})
}

export const saveSystemSettingForm = form => updateSystemSettings({
  announcement: form.announcement,
  geoip_enabled: form.geoip_enabled,
  geoip_city_url: form.geoip_city_url,
  geoip_country_url: form.geoip_country_url,
  geoip_asn_url: form.geoip_asn_url,
  geoip_update_hours: form.geoip_update_hours,
  web_login_hours: form.web_login_hours,
  client_login_hours: form.client_login_hours,
  user_token_retention_days: form.user_token_retention_days,
  login_log_retention_days: form.login_log_retention_days,
  audit_conn_retention_days: form.audit_conn_retention_days,
  audit_file_retention_days: form.audit_file_retention_days,
  control_audit_retention_days: form.control_audit_retention_days,
})
