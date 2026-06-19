import { Link } from "react-router-dom";
import CommandBox from "../components/CommandBox";
import Download from "../components/Download";
import Terminal from "../components/Terminal";

export default function Home() {
  return (
    <>
      <header className="hero">
        <h1>
          tessera <span className="em">— end-to-end encrypted 2FA</span>
        </h1>
        <p className="lede">
          A 2FA authenticator that <strong>never loses an account on sync</strong> and{" "}
          <strong>keeps your recovery codes encrypted</strong> right next to the account
          they belong to. The sync backend only ever sees ciphertext.
        </p>

        <CommandBox command="go install github.com/bm-197/tessera/cmd/tessera@latest" />
        <div className="reqs">Requires Go 1.26+ · CLI + terminal UI in one binary</div>

        <div className="chips">
          <span className="chip">TOTP / HOTP · RFC 6238</span>
          <span className="chip">Argon2id + AES-256-GCM</span>
          <span className="chip">zero-knowledge sync</span>
          <span className="chip">no account loss</span>
        </div>
      </header>

      <Download />

      <section>
        <h2>The terminal UI</h2>
        <Terminal />
      </section>

      <section>
        <h2>Why it exists</h2>
        <ul className="feat">
          <li>
            <strong>Sync never loses an account.</strong> The vault merges instead of
            overwriting — union of all entries, per-field last-write-wins, tombstones for
            deletes. Add one account on a phone and another on a laptop; sync, keep both.
          </li>
          <li>
            <strong>Recovery codes have a home.</strong> Each account stores its backup
            codes, encrypted, in the same place — no more lost screenshots.
          </li>
          <li>
            <strong>The server can't read anything.</strong> Argon2id derives your key;
            the vault is sealed with AES-256-GCM. Backends store only ciphertext.
          </li>
          <li>
            <strong>One core, every client.</strong> A single Go core powers the CLI and
            TUI today — desktop and mobile next — so crypto is written once.
          </li>
        </ul>
      </section>

      <section>
        <h2>Get started</h2>
        <div className="term">
          <pre>
            <span className="t-comment"># create your encrypted vault</span>
            {"\n"}
            <span className="t-cmd">tessera init</span>
            {"\n\n"}
            <span className="t-comment"># add an account from an otpauth:// URI or a setup key</span>
            {"\n"}
            <span className="t-cmd">tessera add "otpauth://totp/GitHub:you?secret=...&issuer=GitHub"</span>
            {"\n\n"}
            <span className="t-comment"># or just open the interactive UI</span>
            {"\n"}
            <span className="t-cmd">tessera tui</span>
          </pre>
        </div>
        <p style={{ marginTop: 14 }}>
          See every command in the{" "}
          <Link to="/docs" style={{ color: "var(--green)" }}>docs</Link>.
        </p>
      </section>
    </>
  );
}
