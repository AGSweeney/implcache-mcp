# ImplCache Librarian frontend

Vite + React + TypeScript source for the Librarian admin UI.

## Dev

```bash
# terminal 1
go run . -db ./implcache.db -http :8080 -enable-librarian -enable-http-mutations -mode admin

# terminal 2
cd frontend
npm install
npm run dev
```

Vite proxies `/api` to `http://127.0.0.1:8080`.

## Production embed

```bash
npm run build
```

Copies `frontend/dist` into `embedui/dist` for `//go:embed`. The Go binary then serves the UI with `-enable-librarian` (no Node.js at runtime).

If `npm install` fails (corporate TLS, etc.), the checked-in `embedui/dist` vanilla SPA remains the production UI and talks to the same `/api/v1` contract.
