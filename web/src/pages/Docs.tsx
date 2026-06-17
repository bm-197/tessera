import { JSX } from "react";

interface Cmd {
  sig: JSX.Element;
  desc: JSX.Element;
}

const commands: Cmd[] = [
  {
    sig: <code className="sig">tessera init</code>,
    desc: <>Create a new encrypted vault for the current profile. Prompts you to set a passphrase (there is no recovery — don't lose it).</>,
  },
  {
    sig: <code className="sig">tessera add "otpauth://..."</code>,
    desc: <>Add an account from an <code>otpauth://</code> URI (TOTP or HOTP). Issuer, label, digits, period and algorithm are read from the URI.</>,
  },
  {
    sig: (
      <code className="sig">
        tessera add <span className="flag">--issuer</span> NAME <span className="flag">--secret</span> KEY{" "}
        [<span className="flag">--label</span> L] [<span className="flag">--digits</span> N] [<span className="flag">--period</span> N] [<span className="flag">--algo</span> A]
      </code>
    ),
    desc: <>Add an account from a manual "setup key". <code>--secret</code> is the base32 key a site shows under "can't scan the QR". Everything else defaults to SHA1 / 6 digits / 30s.</>,
  },
  {
    sig: <code className="sig">tessera list</code>,
    desc: <>List every account with its current code, a countdown, and a <span className="kbd">🔑n</span> badge showing how many recovery codes are stored.</>,
  },
  {
    sig: <code className="sig">tessera get &lt;id|issuer&gt;</code>,
    desc: <>Print just the current code for one account — handy for piping or scripts.</>,
  },
  {
    sig: <code className="sig">tessera recovery set &lt;id&gt; CODE...</code>,
    desc: <>Store one or more backup/recovery codes alongside an account. They're encrypted in the vault like everything else.</>,
  },
  {
    sig: <code className="sig">tessera recovery get &lt;id&gt;</code>,
    desc: <>Show the recovery codes saved for an account.</>,
  },
  {
    sig: <code className="sig">tessera rm &lt;id&gt;</code>,
    desc: <>Delete an account. It's tombstoned, not erased, so the deletion survives a sync instead of being resurrected by a stale copy.</>,
  },
  {
    sig: <code className="sig">tessera sync [<span className="flag">--path</span> FILE]</code>,
    desc: <>Three-way merge your vault with a filesystem backend (e.g. a Dropbox/iCloud/Syncthing folder). The path is remembered after the first run.</>,
  },
  {
    sig: <code className="sig">tessera tui</code>,
    desc: <>Open the full-screen interactive UI (the recommended way to use it day to day).</>,
  },
];

export default function Docs() {
  return (
    <>
      <header className="hero" style={{ paddingBottom: 12 }}>
        <h1>docs</h1>
        <p className="lede">
          Tessera is one binary with a CLI and a TUI. The passphrase is read from{" "}
          <code>$TESSERA_PASSPHRASE</code> if set, otherwise you're prompted.
        </p>
      </header>

      <section>
        <h2>Commands</h2>
        {commands.map((c, i) => (
          <div className="doc-cmd" key={i}>
            {c.sig}
            <p>{c.desc}</p>
          </div>
        ))}
      </section>

      <section>
        <h2>Profiles — multiple vaults</h2>
        <p>
          Each profile is a separate, independently-encrypted vault (different passphrase,
          file, and sync target). Add <code className="sig"><span className="flag">--profile</span> NAME</code>{" "}
          (or <code className="sig"><span className="flag">-p</span> NAME</code>) to any command; omit it for the{" "}
          <code>default</code> profile.
        </p>
        <div className="term">
          <pre>
            <span className="t-cmd">tessera -p work init</span>
            {"\n"}
            <span className="t-cmd">tessera -p work tui</span>
            {"\n"}
            <span className="t-cmd">tessera -p work add "otpauth://..."</span>
          </pre>
        </div>
        <p style={{ marginTop: 12 }} className="note">
          Create a vault with <code>init</code> from the CLI — the TUI only opens existing ones.
        </p>
      </section>

      <section>
        <h2>TUI keys</h2>
        <ul className="feat">
          <li><span className="kbd">↑/↓</span> move · <span className="kbd">enter</span> open an account's details</li>
          <li><span className="kbd">a</span> add (paste a URI or raw secret — name required, label optional)</li>
          <li><span className="kbd">d</span> delete (asks to confirm)</li>
          <li>on the detail screen: <span className="kbd">e</span> edit name/label · <span className="kbd">r</span> add or edit recovery codes</li>
          <li><span className="kbd">s</span> sync · <span className="kbd">q</span> quit</li>
        </ul>
      </section>
    </>
  );
}
