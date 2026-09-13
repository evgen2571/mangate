# Mangate Python wrapper

This package invokes the Mangate command-line executable and returns its structured results. It requires Python 3.10+ and a separately installed Go CLI.

From the repository root:

```sh
go install ./cmd/mangate
uv venv
uv pip install ./python
```

```python
from mangate import Client

client = Client(executable="mangate")
print(client.version())
print(client.providers())
```

Use an absolute path for `executable` if Mangate is not on `PATH`. See the [Python API guide](https://github.com/evgen2571/mangate/blob/main/docs/python.md) for searching, chapter downloads, archive conversion, cancellation, and error behavior.
