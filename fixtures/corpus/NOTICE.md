# Corpus attribution and license

The snapshot files in `snapshots/` are verbatim plaintext extracts of English
Wikipedia articles, licensed under the Creative Commons Attribution-ShareAlike
4.0 International License (CC BY-SA 4.0, https://creativecommons.org/licenses/by-sa/4.0/).
They were retrieved via the Wikipedia API (`action=query&prop=extracts&explaintext=true&exsectionformat=wiki`)
and are reproduced verbatim, unmodified, as frozen bytes.

Attribution: each article's authors are the Wikipedia contributors of the
article at the recorded revision. Per-article publisher, article URL, revision
URL (`?oldid=`), retrieval date, license, byte count and SHA-256 digest are
recorded in `manifest.json`. Example attribution form:

  "Wikipedia contributors, '<title>', English Wikipedia, revision <revision>,
  <revision_url>, licensed CC BY-SA 4.0."

The manifest, selection procedure (`sample.py`, `selection.json`) and
segmentation reference (`segment.py`) are project files, not Wikipedia content.
