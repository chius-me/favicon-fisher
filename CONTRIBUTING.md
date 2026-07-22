# Contributing

## Develop

```bash
# Go
go test ./...
go test -race ./...

# Worker
cd worker
npm ci
npm run check
npm test
npm run test:golden
```

## CI gates

All of CI, Release, Container, and Deploy call the reusable workflow `.github/workflows/verify.yml`:

- Go vet / test / race / short fuzz / govulncheck / embedded `app.js` sync  
- Worker TypeScript check / build / golden / Vitest  

Additional on `main`/PR: CodeQL. Container builds run Trivy (HIGH/CRITICAL) after push.

## Conventions

- Prefer sentinel-safe public errors (`security.PublicError`).
- Never load remote icon URLs in the browser; always signed `/api/proxy`.
- Keep Go and Worker discovery limits aligned (`security/limits.go` vs `worker/src/constants.ts`).
- Pin GitHub Actions to full commit SHAs; Dependabot updates them weekly.

## Security

Report vulnerabilities privately — see [SECURITY.md](./SECURITY.md).
