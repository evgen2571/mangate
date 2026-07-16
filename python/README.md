# Mangate Python API

This package drives the compatible `mangate` executable and returns its stable JSON data as Python dictionaries. Install the Go executable first, or pass its path to `Client`.

```python
from mangate import Client

client = Client()
for provider in client.providers():
    print(provider["info"]["id"])

titles = client.search("mangadex", "public domain", limit=5)
```

The calls block until the executable returns. A `Client` owns no shared mutable state and can be used from multiple Python threads. Each call starts a separate process. Pass a `threading.Event` as `cancel_event` to `download` to terminate its process safely. Already finalized pages remain reusable and incomplete pages remain `.part` files.
