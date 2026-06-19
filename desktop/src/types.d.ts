export {};

type CallResult = { ok: true; data: any } | { ok: false; error: string };

declare global {
  interface Window {
    tessera: {
      call(method: string, params?: unknown): Promise<CallResult>;
      pickSyncFile(): Promise<string | null>;
    };
  }
}
