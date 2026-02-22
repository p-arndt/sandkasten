# Development

## Releasing a new version

### 1. Update version

Edit `pyproject.toml` and bump the `version` field:

```toml
version = "0.2.1"  # or 0.3.0, 1.0.0, etc.
```

### 2. Build

```bash
uv build
```

Creates `dist/sandkasten-<version>.tar.gz` and `dist/sandkasten-<version>-py3-none-any.whl`.

### 3. Upload to PyPI

```bash
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
