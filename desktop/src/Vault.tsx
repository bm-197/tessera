import { useCallback, useEffect, useState } from "react";
import {
  Check,
  Copy,
  Key,
  KeyRound,
  Loader2,
  Lock,
  MoreHorizontal,
  Plus,
  Repeat,
  Trash2,
} from "lucide-react";
import { api, Account, CodeView, displayName, formatCode } from "./lib/client";
import icon from "./assets/icon.png";

const ICON = { size: 16, strokeWidth: 1.75 } as const;

function Ring({ ms, period }: { ms: number; period: number }) {
  const r = 10;
  const circ = 2 * Math.PI * r;
  const frac = Math.max(0, Math.min(1, ms / (period * 1000)));
  const low = ms <= 5000;
  return (
    <svg className="ring" viewBox="0 0 26 26" aria-hidden="true">
      <circle cx="13" cy="13" r={r} fill="none" stroke="var(--border)" strokeWidth="3" />
      <circle
        cx="13"
        cy="13"
        r={r}
        fill="none"
        stroke={low ? "var(--yellow)" : "var(--green)"}
        strokeWidth="3"
        strokeLinecap="round"
        strokeDasharray={circ}
        strokeDashoffset={circ * (1 - frac)}
        transform="rotate(-90 13 13)"
        style={{ transition: "stroke-dashoffset 0.5s linear" }}
      />
    </svg>
  );
}

export default function Vault({ onLock }: { onLock: () => void }) {
  const [codes, setCodes] = useState<CodeView[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [toast, setToast] = useState<{ msg: string; err?: boolean } | null>(null);
  const [adding, setAdding] = useState(false);
  const [detail, setDetail] = useState<Account | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);

  const refresh = useCallback(async () => {
    try {
      setCodes((await api.codes()).codes);
      setLoaded(true);
    } catch {
      /* locked or transient */
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 1000);
    return () => clearInterval(id);
  }, [refresh]);

  function flash(msg: string, err = false) {
    setToast({ msg, err });
    setTimeout(() => setToast(null), 1500);
  }

  async function copy(code: string, id: string) {
    try {
      await navigator.clipboard.writeText(code);
      setCopiedId(id);
      setTimeout(() => setCopiedId((c) => (c === id ? null : c)), 1200);
    } catch {
      flash("copy failed", true);
    }
  }

  async function lock() {
    await api.lock();
    onLock();
  }

  async function sync() {
    const path = await api.pickSyncFile();
    if (!path) return;
    setSyncing(true);
    try {
      await api.sync(path);
      await refresh();
      flash("synced");
    } catch (e) {
      flash((e as Error).message, true);
    } finally {
      setSyncing(false);
    }
  }

  return (
    <div className="app">
      <div className="header">
        <img className="logo" src={icon} alt="" />
        <span className="title">Tessera</span>
        <span className="spacer" />
        <Tooltip label={syncing ? "Syncing…" : "Sync vault"}>
          <button className="iconbtn" onClick={sync} disabled={syncing} aria-label="Sync vault">
            {syncing ? <Loader2 size={15} strokeWidth={1.75} className="spin" /> : <Repeat size={15} strokeWidth={1.75} />}
          </button>
        </Tooltip>
        <Tooltip label="Lock vault">
          <button className="iconbtn" onClick={lock} aria-label="Lock vault">
            <Lock size={15} strokeWidth={1.75} />
          </button>
        </Tooltip>
      </div>

      {!loaded ? (
        <div className="list">
          <div className="skel" />
          <div className="skel" />
          <div className="skel" />
        </div>
      ) : codes.length === 0 ? (
        <div className="empty">
          <div className="emptyicon">
            <KeyRound size={24} strokeWidth={1.5} />
          </div>
          <p>No accounts yet. Add one from an otpauth:// URI or a setup key.</p>
          <button className="btn" onClick={() => setAdding(true)}>
            <Plus {...ICON} /> Add account
          </button>
        </div>
      ) : (
        <div className="list">
          {codes.map((c) => (
            <div className="card" key={c.id}>
              <div className="info">
                <div className="name">{c.issuer || c.label || "Unnamed"}</div>
                {(!!(c.issuer && c.label) || c.recoveryCount > 0) && (
                  <div className="sub">
                    {!!(c.issuer && c.label) && <span className="acct">{c.label}</span>}
                    {c.recoveryCount > 0 && (
                      <span className="keytag">
                        <Key size={12} strokeWidth={1.75} /> {c.recoveryCount}
                      </span>
                    )}
                  </div>
                )}
              </div>
              {c.error ? (
                <span className="code-btn err" title={c.error}>
                  error
                </span>
              ) : (
                <>
                  <button
                    className="code-btn"
                    onClick={() => copy(c.code, c.id)}
                    title="Copy code"
                    aria-label={`Copy code for ${displayName(c)}`}
                  >
                    {formatCode(c.code)}
                    {copiedId === c.id ? (
                      <Check size={15} strokeWidth={2} className="copyicon done" />
                    ) : (
                      <Copy size={15} strokeWidth={1.75} className="copyicon" />
                    )}
                  </button>
                  <Ring ms={c.expiresInMs} period={c.period} />
                </>
              )}
              <button
                className="kebab"
                onClick={() => setDetail(c)}
                title="Details"
                aria-label={`Details for ${displayName(c)}`}
              >
                <MoreHorizontal {...ICON} />
              </button>
            </div>
          ))}
        </div>
      )}

      {loaded && codes.length > 0 && (
        <button className="fab" onClick={() => setAdding(true)} title="Add account" aria-label="Add account">
          <Plus size={24} strokeWidth={2} />
        </button>
      )}

      {adding && (
        <AddModal
          onClose={() => setAdding(false)}
          onAdded={async () => {
            setAdding(false);
            await refresh();
            flash("added");
          }}
        />
      )}

      {detail && (
        <DetailModal
          account={detail}
          onClose={() => setDetail(null)}
          onChanged={async (msg) => {
            await refresh();
            if (msg) flash(msg);
          }}
          onDeleted={async () => {
            setDetail(null);
            await refresh();
            flash("deleted");
          }}
        />
      )}

      {toast && (
        <div className={"toast" + (toast.err ? " err" : "")}>
          {!toast.err && toast.msg !== "copy failed" && <Check size={14} strokeWidth={2} />}
          {toast.msg}
        </div>
      )}
    </div>
  );
}

function AddModal({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const [secret, setSecret] = useState("");
  const [name, setName] = useState("");
  const [label, setLabel] = useState("");
  const [recovery, setRecovery] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const lines = (s: string) =>
    s.split("\n").map((l) => l.trim()).filter(Boolean);

  async function submit() {
    setErr("");
    const raw = secret.trim();
    if (!raw) {
      setErr("Enter a secret or otpauth:// URI.");
      return;
    }
    setBusy(true);
    try {
      if (raw.startsWith("otpauth://")) {
        await api.addURI(raw, lines(recovery));
      } else {
        if (!name.trim()) {
          setErr("Account name is required.");
          setBusy(false);
          return;
        }
        await api.addManual({
          issuer: name.trim(),
          label: label.trim(),
          secret: raw,
          recoveryCodes: lines(recovery),
        });
      }
      onAdded();
    } catch (e) {
      setErr((e as Error).message);
      setBusy(false);
    }
  }

  const isURI = secret.trim().startsWith("otpauth://");

  return (
    <Overlay onClose={onClose}>
      <h2>Add account</h2>
      <label className="fld">
        <span>Secret or otpauth:// URI</span>
        <textarea
          value={secret}
          autoFocus
          spellCheck={false}
          placeholder="otpauth://… or a base32 secret"
          onChange={(e) => setSecret(e.target.value)}
        />
      </label>
      {!isURI && (
        <>
          <label className="fld">
            <span>Account name (required)</span>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. GitHub" />
          </label>
          <label className="fld">
            <span>Label (optional)</span>
            <input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="you@example.com"
            />
          </label>
        </>
      )}
      <label className="fld">
        <span>Recovery codes (optional, one per line)</span>
        <textarea value={recovery} onChange={(e) => setRecovery(e.target.value)} />
      </label>
      {err && <div className="muted" style={{ color: "var(--red)" }}>{err}</div>}
      <div className="row">
        <button className="btn secondary" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button className="btn" disabled={busy} onClick={submit}>
          {busy ? (
            <>
              <Loader2 {...ICON} className="spin" /> Adding…
            </>
          ) : (
            "Add"
          )}
        </button>
      </div>
    </Overlay>
  );
}

function DetailModal({
  account,
  onClose,
  onChanged,
  onDeleted,
}: {
  account: Account;
  onClose: () => void;
  onChanged: (msg?: string) => void;
  onDeleted: () => void;
}) {
  const [issuer, setIssuer] = useState(account.issuer);
  const [label, setLabel] = useState(account.label);
  const [recovery, setRecovery] = useState("");
  const [confirmDel, setConfirmDel] = useState(false);

  useEffect(() => {
    api
      .recoveryCodes(account.id)
      .then((r) => setRecovery(r.codes.join("\n")))
      .catch(() => {});
  }, [account.id]);

  async function saveName() {
    if (!issuer.trim()) return;
    await api.setName(account.id, issuer.trim(), label.trim());
    onChanged("renamed");
  }

  async function saveRecovery() {
    const codes = recovery.split("\n").map((l) => l.trim()).filter(Boolean);
    await api.setRecoveryCodes(account.id, codes);
    onChanged("recovery saved");
  }

  return (
    <Overlay onClose={onClose}>
      <h2>{displayName(account)}</h2>

      <label className="fld">
        <span>Account name</span>
        <input value={issuer} onChange={(e) => setIssuer(e.target.value)} />
      </label>
      <label className="fld">
        <span>Label</span>
        <input value={label} onChange={(e) => setLabel(e.target.value)} />
      </label>
      <div className="row">
        <button className="btn secondary" onClick={saveName}>
          Save name
        </button>
      </div>

      <label className="fld">
        <span>Recovery codes (one per line)</span>
        <textarea value={recovery} onChange={(e) => setRecovery(e.target.value)} />
      </label>
      <div className="row">
        <button className="btn secondary" onClick={saveRecovery}>
          Save codes
        </button>
      </div>

      <div className="row" style={{ justifyContent: "space-between", marginTop: 4 }}>
        {confirmDel ? (
          <>
            <span className="muted" style={{ color: "var(--red)" }}>
              Delete this account?
            </span>
            <span style={{ display: "flex", gap: 8 }}>
              <button className="btn secondary" onClick={() => setConfirmDel(false)}>
                No
              </button>
              <button className="btn danger" onClick={() => api.remove(account.id).then(onDeleted)}>
                Delete
              </button>
            </span>
          </>
        ) : (
          <>
            <button className="btn danger" onClick={() => setConfirmDel(true)}>
              <Trash2 {...ICON} /> Delete
            </button>
            <button className="btn secondary" onClick={onClose}>
              Close
            </button>
          </>
        )}
      </div>
    </Overlay>
  );
}

function Tooltip({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <span className="tip">
      {children}
      <span className="tip-label" role="tooltip">
        {label}
      </span>
    </span>
  );
}

function Overlay({
  children,
  onClose,
}: {
  children: React.ReactNode;
  onClose: () => void;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        {children}
      </div>
    </div>
  );
}
