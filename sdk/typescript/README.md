# Sandkasten TypeScript SDK

This package provides a lightweight, promise‑based client for the Sandkasten HTTP API.  It is shipped separately from the Go daemon and can be installed via npm/pnpm/yarn.

## Installation

```bash
# using pnpm (recommended)
pnpm add @sandkasten/sdk

# or with npm
npm install @sandkasten/sdk
```

## Quick start

```ts
import { SandboxClient } from "@sandkasten/sdk";

async function main() {
  const client = new SandboxClient({ baseUrl: "http://localhost:8080", apiKey: "sk-test" });
  const session = await client.createSession();
  const execResult = await session.exec("echo hello");
  console.log(execResult.output);
}
```

All of the SDK types (`SessionInfo`, `ExecResult`, etc.) are exported from the package and correspond directly to the JSON objects returned by the daemon.  The TypeScript definitions are kept in `src/types.ts` and are intentionally very small; new fields added to the server should be reflected here as needed.  The unit tests included with the repository validate that the client sends the expected requests and decodes responses correctly.

## Development

```bash
cd sdk
pnpm install       # install dev dependencies
pnpm run build      # compile to `dist/`
pnpm run test       # run the unit tests (vitest)
```

Type checking is performed by `tsc` as part of `build`; the project is configured for strict mode.

### Releasing a new version

The monorepo publishes the SDK automatically on a tag push.  The GitHub Actions workflow:

- checks out the repo
- runs `pnpm install`, `pnpm build` and `pnpm test` in `sdk`
- bumps `package.json` to the tagged version and publishes using `pnpm publish`

To manually publish locally you can also run:

```bash
cd sdk
pnpm version 1.2.3 # or use `npm version` to update package.json
pnpm publish --access public
```

Make sure you have an `NPM_TOKEN` environment variable configured or set in the repository secrets so that `pnpm publish` can authenticate.

> **Note:** the SDK is published as a public package.  Use `--access public` with pnpm if your npm registry defaults to restricted packages.

## API compatibility

The TypeScript types mirror the server's API structures.  Any changes to the API should be accompanied by:

1. updating the corresponding TypeScript interfaces in `src/types.ts` (and exporting them in `src/index.ts`);
2. adding or updating unit tests under `sdk/tests/` to cover the new behaviour;
3. incrementing the package version before publishing (the release workflow will set the version from the git tag).

For a deeper guarantee you can run the integration tests against a local daemon (`go test ./tests/integration/...`) and exercise the client from TypeScript in the same environment, but the existing mocks are usually sufficient for catching mismatches.
