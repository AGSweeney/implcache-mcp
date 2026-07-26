import { NavLink, Route, Routes, useLocation } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import EnvironmentStatus from "./components/EnvironmentStatus";
import { ToastProvider } from "./components/Toast";
import Dashboard from "./pages/Dashboard";
import Sources from "./pages/Sources";
import Jobs from "./pages/Jobs";
import Library from "./pages/Library";
import Roots from "./pages/Roots";
import SearchLab from "./pages/SearchLab";
import Health from "./pages/Health";
import Settings from "./pages/Settings";
import Analytics from "./pages/Analytics";
import AddSource from "./pages/AddSource";
import Logs from "./pages/Logs";

const navGroups = [
  {
    label: "Operate",
    links: [
      ["/", "Dashboard"],
      ["/sources", "Sources"],
      ["/sources/add", "Add Source"],
      ["/jobs", "Jobs"],
    ],
  },
  {
    label: "Inspect",
    links: [
      ["/library", "Library"],
      ["/roots", "Roots"],
      ["/search", "Search Lab"],
      ["/analytics", "Analytics"],
      ["/health", "Health"],
    ],
  },
  {
    label: "System",
    links: [
      ["/logs", "Logs"],
      ["/settings", "Settings"],
    ],
  },
] as const;

const pageCrumbs: Record<string, string> = {
  "/": "Dashboard",
  "/sources": "Sources",
  "/sources/add": "Add Source",
  "/jobs": "Jobs",
  "/library": "Library",
  "/roots": "Roots",
  "/search": "Search Lab",
  "/analytics": "Analytics",
  "/health": "Health",
  "/logs": "Logs",
  "/settings": "Settings",
};

function BrandMark() {
  return (
    <div className="mark" aria-hidden="true">
      <svg viewBox="0 0 40 40" width="40" height="40">
        <rect x="2" y="2" width="36" height="36" rx="3" fill="none" stroke="currentColor" strokeWidth="2" />
        <path d="M10 28 V12 h8 a6 6 0 0 1 0 12 h-4 v4 z M22 12 h8 v4 h-8 z M26 20 h4 v8 h-4 z" fill="currentColor" />
      </svg>
    </div>
  );
}

export default function App() {
  const server = useQuery({ queryKey: ["server"], queryFn: api.server, refetchInterval: 15000 });
  const location = useLocation();
  const crumb = pageCrumbs[location.pathname] || "Librarian";

  return (
    <ToastProvider>
      <div className="layout">
        <aside className="rail">
          <div className="brand-block">
            <BrandMark />
            <div className="brand-text">
              <div className="brand-name">ImplCache</div>
              <div className="brand-product">Librarian</div>
            </div>
          </div>
          <p className="brand-tag">Implementation context library</p>
          <nav className="nav" aria-label="Primary">
            {navGroups.map((group) => (
              <div className="nav-group" key={group.label}>
                <div className="nav-label">{group.label}</div>
                {group.links.map(([to, label]) => (
                  <NavLink key={to} to={to} end={to === "/"} className={({ isActive }) => (isActive ? "active" : "")}>
                    {label}
                  </NavLink>
                ))}
              </div>
            ))}
          </nav>
          <div className="rail-foot">
            <div className="rail-foot-block">
              <div className="row-dot">
                <span className={`env-dot ${server.data && !server.isError ? "ok" : ""}`} aria-hidden="true" />
                <span>
                  {server.isError && "offline"}
                  {server.data &&
                    `${server.data.serverVersion} · schema ${server.data.schemaVersion} · ${server.data.role || "—"}`}
                  {!server.data && !server.isError && "…"}
                </span>
              </div>
            </div>
          </div>
        </aside>
        <div className="workspace">
          <header className="topbar">
            <div className="topbar-title">
              <span className="topbar-crumb">Librarian / {crumb}</span>
            </div>
            <EnvironmentStatus server={server.data} error={server.isError} />
          </header>
          <main className="main">
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/sources" element={<Sources />} />
              <Route path="/sources/add" element={<AddSource />} />
              <Route path="/library" element={<Library />} />
              <Route path="/roots" element={<Roots />} />
              <Route path="/search" element={<SearchLab />} />
              <Route path="/analytics" element={<Analytics />} />
              <Route path="/jobs" element={<Jobs />} />
              <Route path="/health" element={<Health />} />
              <Route path="/logs" element={<Logs />} />
              <Route path="/settings" element={<Settings />} />
            </Routes>
          </main>
        </div>
      </div>
    </ToastProvider>
  );
}
