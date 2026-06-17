// A faithful, static recreation of the Tessera TUI account list.
export default function Terminal() {
  return (
    <div className="term">
      <pre>
        <span className="t-title">Tessera</span> <span className="t-dim">·  default</span>
        {"\n\n"}
        <span className="t-cur">▸ GitHub (you)</span>
        {"                "}
        <span className="t-code">003 215</span> <span className="t-code">████░░░░</span>{" "}
        <span className="t-dim">28s</span>
        {"\n\n"}
        {"  Stripe (you@example.com)    "}
        <span className="t-code">159 691</span> <span className="t-code">████░░░░</span>{" "}
        <span className="t-dim">28s</span>  🔑1
        {"\n\n"}
        {"  Fastmail (you)              "}
        <span className="t-code">997 153</span> <span className="t-code">████░░░░</span>{" "}
        <span className="t-dim">28s</span>
        {"\n\n"}
        <span className="t-dim">────────────────────────────────────────────</span>
        {"\n"}
        <span className="t-dim">↑/↓ move · </span>
        <span className="t-key">enter</span>
        <span className="t-dim"> details · </span>
        <span className="t-key">a</span>
        <span className="t-dim"> add · </span>
        <span className="t-key">s</span>
        <span className="t-dim"> sync · </span>
        <span className="t-key">q</span>
        <span className="t-dim"> quit</span>
      </pre>
    </div>
  );
}
