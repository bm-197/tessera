import { useEffect, useState } from "react";

const REL = "https://github.com/bm-197/tessera/releases/latest/download";
const RELEASES = "https://github.com/bm-197/tessera/releases";

type OS = "mac" | "win" | "linux";

const DOWNLOADS: Record<OS, { label: string; file: string; note: string }> = {
  mac: { label: "Download for macOS", file: "Tessera-mac-arm64.dmg", note: "Apple Silicon · .dmg" },
  win: { label: "Download for Windows", file: "Tessera-win-x64.exe", note: "64-bit · .exe" },
  linux: { label: "Download for Linux", file: "Tessera-linux-x86_64.AppImage", note: "64-bit · AppImage" },
};

function detectOS(): OS {
  const s = (navigator.platform + " " + navigator.userAgent).toLowerCase();
  if (s.includes("win")) return "win";
  if (s.includes("linux") || s.includes("android")) return "linux";
  return "mac";
}

export default function Download() {
  const [os, setOS] = useState<OS>("mac");
  useEffect(() => setOS(detectOS()), []);

  const primary = DOWNLOADS[os];
  const others = (Object.keys(DOWNLOADS) as OS[]).filter((k) => k !== os);

  return (
    <section>
      <h2>Desktop app</h2>
      <a className="dl-primary" href={`${REL}/${primary.file}`}>
        {primary.label}
        <span className="dl-note">{primary.note}</span>
      </a>
      <div className="dl-others">
        {others.map((k) => (
          <a key={k} href={`${REL}/${DOWNLOADS[k].file}`}>
            {DOWNLOADS[k].label.replace("Download for ", "")}
          </a>
        ))}
        <a href={RELEASES}>all releases</a>
      </div>
      <p className="note" style={{ marginTop: 12 }}>
        Unsigned for now, so the first launch needs a manual approve: macOS — right-click the
        app and choose Open; Windows — More info, then Run anyway.
      </p>
    </section>
  );
}
