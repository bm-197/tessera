import { Navigate, NavLink, Route, Routes } from "react-router-dom";
import Home from "./pages/Home";
import Docs from "./pages/Docs";

const GITHUB = "https://github.com/bm-197/tessera";

function Nav() {
  return (
    <nav className="top">
      <NavLink to="/" className="brand" end>
        <img className="brand-mark" src="/icon.png" alt="" />
        tessera
      </NavLink>
      <span className="ver">v0.1.0</span>
      <span className="spacer" />
      <NavLink to="/docs">docs</NavLink>
      <a href={GITHUB} aria-label="GitHub repository">
        <img className="gh-icon" src="/github.png" alt="GitHub" width={18} height={18} />
      </a>
    </nav>
  );
}

function TagMark() {
  return (
    <svg className="ic" viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
      <path d="M1 7.775V2.75C1 1.784 1.784 1 2.75 1h5.025c.464 0 .91.184 1.238.513l6.25 6.25a1.75 1.75 0 0 1 0 2.474l-5.026 5.026a1.75 1.75 0 0 1-2.474 0l-6.25-6.25A1.752 1.752 0 0 1 1 7.775Zm1.5 0c0 .066.026.13.073.177l6.25 6.25a.25.25 0 0 0 .354 0l5.025-5.025a.25.25 0 0 0 0-.354l-6.25-6.25a.25.25 0 0 0-.177-.073H2.75a.25.25 0 0 0-.25.25ZM6 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Z" />
    </svg>
  );
}

function Footer() {
  return (
    <footer>
      <span>
        built by <a href="https://github.com/bm-197">bm-197</a>
      </span>
      <span className="spacer" />
      <a href={GITHUB} aria-label="Source on GitHub">
        <img className="gh-icon" src="/github.png" alt="GitHub" width={16} height={16} />
      </a>
      <a href={`${GITHUB}/releases`} aria-label="Releases">
        <TagMark />
      </a>
      <NavLink to="/docs">docs</NavLink>
    </footer>
  );
}

export default function App() {
  return (
    <div className="wrap">
      <Nav />
      <main>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/docs" element={<Docs />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
      <Footer />
    </div>
  );
}
