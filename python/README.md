# Mangate Python API

This package drives a compatible `mangate` executable and returns its stable JSON data as Python dictionaries. Install the Go executable first, or pass its path to `Client`.

Install it from the repository with:

```bash
python -m pip install ./python
```

```python
from mangate import Client

client = Client(output_format="cbz")
for provider in client.providers():
    print(provider["info"]["id"])

titles = client.search("public domain", provider="mangadex", limit=5)

# Convert an existing page directory without contacting a provider.
converted = client.convert("./library/Example-123/Chapter-1", output_format="zip")
```

The calls block until the executable returns. A `Client` owns no shared mutable state and can be used from multiple Python threads. Each call starts a separate process. Pass a `threading.Event` as `cancel_event` to `download` to terminate its process safely. Already finalized pages remain reusable and incomplete pages remain `.part` files.

For the complete API reference, see the [Python bindings guide](../docs/python.md).
