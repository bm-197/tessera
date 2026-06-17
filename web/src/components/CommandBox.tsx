import { useState } from "react";

export default function CommandBox({ command }: { command: string }) {
  const [label, setLabel] = useState("copy");

  async function copy() {
    try {
      await navigator.clipboard.writeText(command);
      setLabel("copied");
      setTimeout(() => setLabel("copy"), 1500);
    } catch {
      setLabel("⌘C");
    }
  }

  return (
    <div className="cmd">
      <code>
        <span className="sigil">$ </span>
        {command}
      </code>
      <button onClick={copy} aria-label="Copy command">
        {label}
      </button>
    </div>
  );
}
