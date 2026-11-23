# Troubleshooting

Start with `mangate diagnostics` if you aren't sure what's wrong. It checks your local setup without contacting MangaDex.

| What you see | What to try |
| --- | --- |
| No search results | Check the title spelling or remove language and content filters. An empty result isn't a network error. |
| A query exceeds the provider's result window | Narrow the search. `--limit` only caps what's shown. |
| MangaDex rate-limits requests | Wait before retrying. Mangate already retries rate limits and some temporary server errors. |
| Requests time out | Try a longer `http.timeout` in your [config file](configuration.md), such as `1m`. |
| A chapter number matches several releases | Run `mangate chapters TITLE_ID --chapter-language en` and copy the ID of the release you want. |
| The output can't be written | Check free space and permissions, and make sure no file is in the way of the output directory. |
| The TUI doesn't fit or colors are hard to read | Use a real terminal at least 48 columns by 14 rows. Widen it for the cover and synopsis, or try `--no-color`. |

## Resume a download

Run the same command with the same output directory. The default `--existing-files skip` reuses matching pages. Keep the chapter's `.mangate.json`; it records whether the chapter finished. If you're changing formats or replacing a bad archive, use a new destination or opt in with `--existing-files replace`.

For a failed archive download, keep the source pages and fix the archive error before retrying. To check an existing archive, run `mangate archive verify ./chapter.cbz`.

Press **Ctrl+C** to cancel a command, or **q** to cancel an active TUI download. Pages that already finished are kept for the next run.

## Python can't find Mangate

Platform wheels include the executable. For editable installs, build the CLI and put it on `PATH`, or pass `executable="/absolute/path/to/mangate"` (use `mangate.exe` on Windows). Try `mangate.cli_version()` from the same Python environment to check discovery.
