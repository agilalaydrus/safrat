// The impersonation token, held in the browser tab that started it.
//
// sessionStorage rather than localStorage on purpose: a session that survives
// closing the tab is a session somebody forgets about. The server expires it
// too, but the two ought to agree about what "finished" means.
//
// The token is a credential. It is never put in a URL, never logged, and never
// sent anywhere except this API's own Connect endpoint.
const KEY = "tawafiqhub.impersonation";

export interface ImpersonationState {
  token: string;
  operatorId: string;
  operatorName: string;
  expiresAt: number;
}

function read(): ImpersonationState | undefined {
  if (typeof window === "undefined") return undefined;
  try {
    const raw = window.sessionStorage.getItem(KEY);
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as ImpersonationState;
    // An expired token is worse than none: it makes every call fail with an
    // authentication error that looks like the admin's own session broke.
    if (!parsed.token || parsed.expiresAt <= Date.now()) {
      window.sessionStorage.removeItem(KEY);
      return undefined;
    }
    return parsed;
  } catch {
    // Private browsing, cleared storage, or a value from an older shape.
    return undefined;
  }
}

export function currentImpersonation(): ImpersonationState | undefined {
  return read();
}

export function impersonationToken(): string | undefined {
  return read()?.token;
}

export function startImpersonationLocally(state: ImpersonationState) {
  try {
    window.sessionStorage.setItem(KEY, JSON.stringify(state));
  } catch {
    // Nothing stored means the banner and header will not appear, which is the
    // safe direction: no silent impersonation.
  }
}

export function clearImpersonation() {
  try {
    window.sessionStorage.removeItem(KEY);
  } catch {
    // Ignored for the same reason as above.
  }
}
