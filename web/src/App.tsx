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

function Footer() {
  return (
    <footer>
      <span>
        built by <a href="https://github.com/bm-197">bm-197</a>
      </span>
      <span className="spacer" />
      <a href={GITHUB}>source</a>
      <a href={`${GITHUB}/releases`}>releases</a>
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
