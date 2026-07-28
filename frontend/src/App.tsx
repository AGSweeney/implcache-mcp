import { useEffect, useState, type ReactNode } from "react";
import { NavLink, Route, Routes, useLocation } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import BrandLogo from "./components/BrandLogo";
import Button from "./components/Button";
import EnvironmentStatus from "./components/EnvironmentStatus";
import ErrorBoundary from "./components/ErrorBoundary";
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

const COLLAPSE_KEY = "librarian.sidebarCollapsed";

type NavLinkDef = { to: string; label: string; icon: ReactNode };

const navGroups: { label: string; links: NavLinkDef[] }[] = [
  {
    label: "Operate",
    links: [
      { to: "/", label: "Dashboard", icon: <IconDashboard /> },
      { to: "/sources", label: "Sources", icon: <IconSources /> },
      { to: "/jobs", label: "Jobs", icon: <IconJobs /> },
    ],
  },
  {
    label: "Inspect",
    links: [
      { to: "/library", label: "Library", icon: <IconLibrary /> },
      { to: "/roots", label: "Roots", icon: <IconRoots /> },
      { to: "/search", label: "Search Lab", icon: <IconSearch /> },
      { to: "/analytics", label: "Analytics", icon: <IconAnalytics /> },
      { to: "/health", label: "Health", icon: <IconHealth /> },
    ],
  },
  {
    label: "System",
    links: [
      { to: "/logs", label: "Logs", icon: <IconLogs /> },
      { to: "/settings", label: "Settings", icon: <IconSettings /> },
    ],
  },
];

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

function IconDashboard() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <rect x="3.5" y="3.5" width="7" height="7" rx="1.2" />
      <rect x="13.5" y="3.5" width="7" height="7" rx="1.2" />
      <rect x="3.5" y="13.5" width="7" height="7" rx="1.2" />
      <rect x="13.5" y="13.5" width="7" height="7" rx="1.2" />
    </svg>
  );
}
function IconSources() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <ellipse cx="12" cy="6" rx="7.5" ry="2.8" />
      <path d="M4.5 6v6c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8V6" />
      <path d="M4.5 12v6c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8v-6" />
    </svg>
  );
}
function IconJobs() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M3.5 12h4l2-5.5L13.5 17l2.5-5H20.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconLibrary() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M5 4.5h4v15H5zM10.5 4.5h4v15h-4zM16 6.5c2.2-.8 3.5-.5 3.5 1.8v9.2c0 1.8-1.4 2.2-3.5 1.4" strokeLinejoin="round" />
    </svg>
  );
}
function IconRoots() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 4v7M12 11l-5 8M12 11l5 8M7 11h10" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconSearch() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <circle cx="11" cy="11" r="6.5" />
      <path d="M16 16l4 4" strokeLinecap="round" />
      <path d="M9 10.5h4M11 8.5v4" strokeLinecap="round" />
    </svg>
  );
}
function IconAnalytics() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M4 19h16M7 16V9M12 16V5M17 16v-4" strokeLinecap="round" />
    </svg>
  );
}
function IconHealth() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 3.5 5.5 6v5.5c0 4.2 2.8 7.6 6.5 9 3.7-1.4 6.5-4.8 6.5-9V6L12 3.5Z" strokeLinejoin="round" />
      <path d="M9.5 12.2 11.2 14l3.4-3.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconLogs() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6 5.5h12v13H6z" />
      <path d="M9 9h6M9 12.5h6M9 16h4" strokeLinecap="round" />
    </svg>
  );
}
function IconSettings() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path d="M12 3.5v2.2M12 18.3v2.2M3.5 12h2.2M18.3 12h2.2M6 6l1.6 1.6M16.4 16.4 18 18M18 6l-1.6 1.6M7.6 16.4 6 18" strokeLinecap="round" />
    </svg>
  );
}
function IconCollapse({ collapsed }: { collapsed: boolean }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      {collapsed ? (
        <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
      ) : (
        <path d="M15 6l-6 6 6 6" strokeLinecap="round" strokeLinejoin="round" />
      )}
    </svg>
  );
}

export default function App() {
  const server = useQuery({ queryKey: ["server"], queryFn: api.server, refetchInterval: 15000 });
  const location = useLocation();
  const crumb = pageCrumbs[location.pathname] || "Librarian";
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return sessionStorage.getItem(COLLAPSE_KEY) === "1";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      sessionStorage.setItem(COLLAPSE_KEY, collapsed ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, [collapsed]);

  return (
    <ToastProvider>
      <div className={`layout ${collapsed ? "is-rail-collapsed" : ""}`}>
        <aside className="rail">
          <div className="brand-block">
            <BrandLogo variant={collapsed ? "mark" : "full"} />
            {!collapsed && <div className="brand-product">Librarian</div>}
          </div>
          {!collapsed && <p className="brand-tag">Implementation context library</p>}
          <nav className="nav" aria-label="Primary">
            {navGroups.map((group) => (
              <div className="nav-group" key={group.label}>
                <div className="nav-label">{group.label}</div>
                {group.links.map((link) => (
                  <NavLink
                    key={link.to}
                    to={link.to}
                    end={link.to === "/"}
                    title={link.label}
                    className={({ isActive }) => (isActive ? "active" : "")}
                  >
                    <span className="nav-icon">{link.icon}</span>
                    <span className="nav-link-label">{link.label}</span>
                  </NavLink>
                ))}
              </div>
            ))}
          </nav>
          <Button
            variant="ghost"
            className="rail-collapse"
            onClick={() => setCollapsed((v) => !v)}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            <span className="nav-icon">
              <IconCollapse collapsed={collapsed} />
            </span>
            <span className="rail-collapse-label">{collapsed ? "Expand" : "Collapse"}</span>
          </Button>
          <div className="rail-foot">
            <div className="rail-foot-block">
              {(() => {
                const statusText = server.isError
                  ? "offline"
                  : server.data
                    ? `${server.data.serverVersion} · schema ${server.data.schemaVersion} · ${server.data.role || "—"}`
                    : "…";
                return (
                  <div className="row-dot" title={statusText}>
                    <span className={`env-dot ${server.data && !server.isError ? "ok" : ""}`} aria-hidden="true" />
                    <span className="rail-foot-text">{statusText}</span>
                  </div>
                );
              })()}
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
            <ErrorBoundary>
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
            </ErrorBoundary>
          </main>
        </div>
      </div>
    </ToastProvider>
  );
}
