import TesseraCore from "../../modules/tessera-core";

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

let cachedVaultPath: string | null = null;
function vaultPath(): string {
  if (!cachedVaultPath) {
    cachedVaultPath = `${TesseraCore.documentsPath()}/tessera-vault.json`;
  }
  return cachedVaultPath;
}

// The native call resolves with the bridge's JSON result string, or rejects
// with a core error. Unlike desktop, there's no {ok,data} envelope here.
async function call<T = unknown>(method: string, params?: object): Promise<T> {
  const out = await TesseraCore.call(method, params ? JSON.stringify(params) : "{}");
  return JSON.parse(out) as T;
}

export const api = {
  status: () => call<{ unlocked: boolean }>("status"),
  vaultExists: () => call<{ exists: boolean }>("vaultExists", { path: vaultPath() }),
  create: (passphrase: string) => call("create", { path: vaultPath(), passphrase }),
  open: (passphrase: string) => call("open", { path: vaultPath(), passphrase }),
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
  recoveryCodes: (id: string) => call<{ codes: string[] }>("recoveryCodes", { id }),
  setName: (id: string, issuer: string, label: string) =>
    call("setName", { id, issuer, label }),
  remove: (id: string) => call("delete", { id }),
};

export function displayName(a: Account): string {
  return a.issuer || a.label || "Unnamed";
}

export function formatCode(code: string): string {
  if (code.length === 6) return `${code.slice(0, 3)} ${code.slice(3)}`;
  if (code.length === 8) return `${code.slice(0, 4)} ${code.slice(4)}`;
  return code;
}
