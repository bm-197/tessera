import { FormEvent, useState } from "react";
import { Loader2 } from "lucide-react";
import { api } from "./lib/client";
import icon from "./assets/icon.png";

export default function Unlock({
  exists,
  onUnlocked,
}: {
  exists: boolean;
  onUnlocked: () => void;
}) {
  const [pass, setPass] = useState("");
  const [confirm, setConfirm] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    if (!pass) return;
    if (!exists && pass !== confirm) {
      setErr("passphrases do not match");
      return;
    }
    setBusy(true);
    try {
      if (exists) await api.open(pass);
      else await api.create(pass);
      onUnlocked();
    } catch (e) {
      setErr(exists ? "wrong passphrase" : (e as Error).message);
      setBusy(false);
    }
  }

  return (
    <div className="app">
      <form className="unlock" onSubmit={submit}>
        <div className={"unlock-inner" + (err ? " shake" : "")} key={err || "ok"}>
          <img src={icon} alt="" />
          <h1>Tessera</h1>
          <p>
            {exists
              ? "Enter your passphrase to unlock."
              : "Create a vault. Your passphrase has no recovery, so keep it safe."}
          </p>
          <input
            className="field-full"
            type="password"
            placeholder="passphrase"
            value={pass}
            autoFocus
            disabled={busy}
            onChange={(e) => setPass(e.target.value)}
          />
          {!exists && (
            <input
              className="field-full"
              type="password"
              placeholder="confirm passphrase"
              value={confirm}
              disabled={busy}
              onChange={(e) => setConfirm(e.target.value)}
            />
          )}
          <div className="err" aria-live="polite">
            {err}
          </div>
          <button className="btn field-full" disabled={busy || !pass}>
            {busy ? (
              <>
                <Loader2 size={16} className="spin" />
                {exists ? "Unlocking…" : "Creating…"}
              </>
            ) : exists ? (
              "Unlock"
            ) : (
              "Create vault"
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
