export interface FormDisabledState {
  readonly account: boolean;
  readonly connection: boolean;
}

export function formDisabledState(active: boolean, busy: boolean, accountGrantDelivered: boolean): FormDisabledState {
  return {
    account: active || busy || accountGrantDelivered,
    connection: active || busy,
  };
}
