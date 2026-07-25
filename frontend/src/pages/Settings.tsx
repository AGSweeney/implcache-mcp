import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api, getToken, setToken } from "../api";

export default function Settings() {
  const server = useQuery({ queryKey: ["server"], queryFn: api.server });
  const [token, setTok] = useState(getToken());

  return (
    <div>
      <h1>Settings</h1>
      <div className="panel stack">
        <h2>Connection</h2>
        <pre>{JSON.stringify(server.data, null, 2)}</pre>
      </div>
      <div className="panel stack">
        <h2>API token</h2>
        <p className="muted">
          When the server is started with <span className="mono">-librarian-token</span>, paste the bearer token here.
          Stored only in this browser&apos;s localStorage.
        </p>
        <label>
          Bearer token
          <input type="password" value={token} onChange={(e) => setTok(e.target.value)} autoComplete="off" />
        </label>
        <div className="row">
          <button
            className="primary"
            type="button"
            onClick={() => {
              setToken(token.trim());
              server.refetch();
            }}
          >
            Save token
          </button>
          <button
            type="button"
            onClick={() => {
              setTok("");
              setToken("");
              server.refetch();
            }}
          >
            Clear
          </button>
        </div>
      </div>
    </div>
  );
}
