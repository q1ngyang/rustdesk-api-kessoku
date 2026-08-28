const pageUUID = crypto.randomUUID();

export interface WebDeviceIdentity { readonly device_id: string; readonly uuid: string; readonly platform: string }

export function webDeviceIdentity(): WebDeviceIdentity {
  const platform = navigator.platform || 'Web';
  const browser = /edg/i.test(navigator.userAgent) ? 'Edge' : /chrome|crios/i.test(navigator.userAgent) ? 'Chrome' : /firefox|fxios/i.test(navigator.userAgent) ? 'Firefox' : /safari/i.test(navigator.userAgent) ? 'Safari' : 'Web browser';
  return { device_id: `${browser} · ${platform}`.slice(0, 128), uuid: pageUUID, platform: String(platform).slice(0, 64) };
}
