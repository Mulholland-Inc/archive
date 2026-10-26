# Attachment-reader compatibility

The downstream archive-family engine copy includes an attachment reader (`src/read.go`) that is not present on this repository's current main branch. The Office extraction, ZIP bounds, original-preserving retry API, and text-only evidence guard in this change compile against main.

`attachment-reader-evidence.patch` records the corresponding generic fixes for that newer reader. It is a prerequisite-dependent patch, not code compiled by this change. Apply it after the attachment reader is introduced, resolving its contexts against that implementation. It does not transplant the reader, family ontology, or unrelated engine features.

The patch bounds disk attachment reads to 20 MiB plus one byte; refuses model calls without text or actual usable attachment bytes; refuses confirmed encrypted originals; and keeps the existing small PDF/image attachment path, including image formats converted to JPEG. Its regression intercepts every model request locally and covers missing and oversized attachments, usable extracted text, small attachment bytes, and recording transcripts.

The ordinary compiled `TestEncryptedFileEvidenceNeverReachesTheReader` also applies to main's text-only reader. The downstream attachment version failed before its narrow encrypted-status guard (one intercepted request), now passes with zero requests. No model service is contacted by these tests.
