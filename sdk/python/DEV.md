# Development

## Releasing a new version

### Automated publish via GitHub Actions (recommended)

PyPI publish is handled by `.github/workflows/main.yaml` on tag pushes.

1. Bump `project.version` in `pyproject.toml`.
2. Commit and push.
3. Create and push a matching tag:

```bash
git tag 0.3.1
git push origin 0.3.1
```

The workflow verifies that the tag matches `pyproject.toml`, builds the package, and publishes to PyPI.

### One-time PyPI setup for CI

Configure a trusted publisher in PyPI for this repository:

- Owner/repo: `p-arndt/sandkasten`
- Workflow: `main.yaml`
- Environment: *(leave empty unless you use one)*

If you prefer token-based publishing, add a `PYPI_API_TOKEN` secret and switch the publish step to token auth.

### Manual publish (fallback)

```bash
uv build
uvx twine upload dist/*
```

Use `--repository testpypi` for Test PyPI first.

---

## Optional: bump dependencies

To update minimum dependency versions to match the lock file:

```bash
uv lock
uvx uv-bump
```
