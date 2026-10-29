# Demo vault

A made-up personal archive to try Archive with: 18 tables (people, places, payments, events, tasks, notes, health, ...), about 820 rows and 35 files (PDFs, emails, photos with EXIF, a bank CSV, a calendar, and a docx, xlsx and pptx to edit). Rent, trips, recipes and other shared things are in a `family` space, shared with a second user, sanne@example.com.

Load it into an empty archive:

```sh
cd src && go build -o archive .
ARCHIVE_DATA=../data ARCHIVE_BIN=./archive ../demo/load.sh you@example.com "Your Name"
```

The email is the first owner, created when the archive has no users yet. Then `archive serve` and connect an agent to `/mcp` with a token from `archive token create -full`.
