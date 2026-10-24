# Vue applications

All six applications use Vue 3 / TypeScript / Vite. Ports: user 5173, admin 5174, partner 5175, developer 5176, creator 5177, ads 5178. Set `VITE_API_URL` if the backend is not `http://localhost:8080`.

From repository root:

```bash
npm install
npm run dev:user
npm run dev:admin
npm run dev:partner
npm run dev:developer
npm run dev:creator
npm run dev:ads
```

The clients are minimal working UI for implemented backend endpoints, **not** full LINE-equivalent UX or finished business modules. Browser access tokens are intentionally kept in memory; production requires session lifecycle, OIDC and refresh-token hardening.
