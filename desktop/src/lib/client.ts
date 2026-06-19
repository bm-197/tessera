export interface Account {
  id: string;
  issuer: string;
  label: string;
  type: string;
  algorithm: string;
  digits: number;
  period: number;
  recoveryCount: number;
}

export interface CodeView extends Account {
  code: string;
  expiresInMs: number;
  error?: string;
}

async function call<T = unknown>(method: string, params?: unknown): Promise<T> {
  const r = await window.tessera.call(method, params);
  if (!r.ok) throw new Error(r.error);
  return r.data as T;
}

export const api = {
  status: () => call<{ unlocked: boolean }>("status"),
  vaultExists: () => call<{ exists: boolean }>("vaultExists"),
  create: (passphrase: string) => call("create", { passphrase }),
  open: (passphrase: string) => call("open", { passphrase }),
  lock: () => call("lock"),
  list: () => call<{ accounts: Account[] }>("list"),
  codes: () => call<{ codes: CodeView[] }>("codes"),
  addURI: (uri: string, recoveryCodes: string[]) =>
    call<{ account: Account }>("addURI", { uri, recoveryCodes }),
  addManual: (p: {
    issuer: string;
    label: string;
    secret: string;
    recoveryCodes: string[];
  }) => call<{ account: Account }>("addManual", p),
  setRecoveryCodes: (id: string, codes: string[]) =>
    call("setRecoveryCodes", { id, codes }),
  recoveryCodes: (id: string) =>
    call<{ codes: string[] }>("recoveryCodes", { id }),
  setName: (id: string, issuer: string, label: string) =>
    call("setName", { id, issuer, label }),
  remove: (id: string) => call("delete", { id }),
  sync: (path: string) => call("sync", { path }),
  pickSyncFile: () => window.tessera.pickSyncFile(),
};

export function displayName(a: Account): string {
  if (a.issuer && a.label) return `${a.issuer} (${a.label})`;
  return a.issuer || a.label || "(unnamed)";
}

export function formatCode(code: string): string {
  if (code.length === 6) return `${code.slice(0, 3)} ${code.slice(3)}`;
  if (code.length === 8) return `${code.slice(0, 4)} ${code.slice(4)}`;
  return code;
}
