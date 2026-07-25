import { NavLink, Route, Routes } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import Dashboard from "./pages/Dashboard";
import Sources from "./pages/Sources";
import Jobs from "./pages/Jobs";
import Library from "./pages/Library";
import Roots from "./pages/Roots";
import SearchLab from "./pages/SearchLab";
import Health from "./pages/Health";
import Settings from "./pages/Settings";
import AddSource from "./pages/AddSource";
import Logs from "./pages/Logs";

const links = [
  ["/", "Dashboard"],
  ["/sources", "Sources"],
  ["/sources/add", "Add Source"],
  ["/library", "Library"],
  ["/roots", "Roots"],
  ["/search", "Search Lab"],
  ["/jobs", "Jobs"],
  ["/health", "Health"],
  ["/logs", "Logs"],
  ["/settings", "Settings"],
] as const;

export default function App() {
  const server = useQuery({ queryKey: ["server"], queryFn: api.server, refetchInterval: 15000 });

  return (
    <div className="layout">
      <header className="topbar">
        <div className="brand">ImplCache Librarian</div>
        <div className="conn">
          {server.isError && <span className="badge err">disconnected</span>}
          {server.data && (
            <>
              <span className="badge ok">connected</span>
              <span>server {server.data.serverVersion}</span>
              <span>api v{server.data.apiVersion}</span>
              <span>schema {server.data.schemaVersion}</span>
              <span>{server.data.readOnly ? "read-only" : "read-write"}</span>
              <span>auth {server.data.authMode}</span>
              {server.data.role && <span>role {server.data.role}</span>}
            </>
          )}
        </div>
      </header>
      <nav className="nav">
        {links.map(([to, label]) => (
          <NavLink key={to} to={to} end={to === "/"} className={({ isActive }) => (isActive ? "active" : "")}>
            {label}
          </NavLink>
        ))}
      </nav>
      <main className="main">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/sources" element={<Sources />} />
          <Route path="/sources/add" element={<AddSource />} />
          <Route path="/library" element={<Library />} />
          <Route path="/roots" element={<Roots />} />
          <Route path="/search" element={<SearchLab />} />
          <Route path="/jobs" element={<Jobs />} />
          <Route path="/health" element={<Health />} />
          <Route path="/logs" element={<Logs />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </main>
    </div>
  );
}
