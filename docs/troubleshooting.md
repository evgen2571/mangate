# Troubleshooting

Start with local diagnostics and effective configuration:

```sh
mangate diagnostics
mangate config
mangate --verbose --json search "title"
```

Diagnostics checks local setup without contacting a provider. Verbose mode adds an error category to stderr; JSON mode provides structured command results on stdout. Configuration-loading failures occur before the command starts and are written to stderr.

## Network and API errors

Check the provider's availability and your connection. Mangate retries rate limits and selected transient server errors a bounded number of times. It honors retry delays and cancels waiting requests when their context ends. Repeatedly restarting a rate-limited command can make matters worse; wait before trying again.

A timeout can occur during a request or while waiting to retry. Adjust `http.timeout` in the configuration file if the connection needs more time. Malformed JSON, API error responses, missing page locations, and invalid covers are reported as errors rather than successful empty responses.

Search and chapter lists are paginated. If a query exceeds the provider's result window, narrow the query. An empty search is reported separately from a failed request and exits with status 1.

## Formats and files

Use `directory` to preserve downloaded image encodings, `png` or `jpeg` to convert pages, and `cbz` or `zip` for chapter archives. JPEG conversion flattens transparency onto white. Dataset collection accepts image-directory formats rather than archives.

Check that the output directory exists or can be created by your account. A file at the requested directory path, a read-only destination, or insufficient storage can prevent output. Error messages include relevant paths; inspect the first underlying filesystem error.

Use `--existing-files skip` to reuse output, `fail` to refuse existing output, or `replace` to replace it. If an existing page's encoding differs from an explicitly requested PNG/JPEG format, choose a separate destination or use replacement. Invalid or mismatched existing archives require explicit replacement.

## Partial and interrupted downloads

Completed pages remain in the chapter directory after a failed transfer. `.mangate.json` records chapter completion state. Retry the same selection and output directory with the default skip policy to reuse completed pages. Failed replacements preserve the old page until the new transfer is finalized.

An archive is created only from a completed chapter. Its source page directory is removed after successful archive validation. If archive creation fails, keep the source directory and resolve the error before retrying. Inspect and verify an archive directly:

```sh
mangate archive inspect ./chapter.cbz
mangate archive verify ./chapter.cbz
```

Ctrl+C cancels a CLI operation. In the TUI, `q` cancels an active download and shows its result. A failed search or chapter request returns to an editable screen with the error visible. Cover failures do not prevent chapter navigation.

## Terminal display

Run `mangate tui` in an interactive terminal, not a redirected pipeline. The interface requires at least 48 columns and 14 rows. Widen the terminal to see the selected result's cover and synopsis. Try `--no-color` if terminal colors are unreadable, or `--color` when color detection fails. The two flags cannot be combined.

## Python executable discovery

The Python package does not install the Go executable. Confirm `mangate --version` works in the same environment that launches Python, or pass the executable path explicitly:

```python
from mangate import Client

client = Client(executable="/absolute/path/to/mangate")
print(client.version())
```

On Windows, point to `mangate.exe`. A missing executable raises an OS-level subprocess error. Command failures represented by the CLI JSON protocol raise `MangateError`, with `category`, `message`, and `stderr` fields. Partial download results are returned as data so callers can inspect chapter outcomes.
