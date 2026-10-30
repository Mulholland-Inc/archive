#!/bin/sh
set -eu
sed -i "s|<meta name=\"archive-api\" content=\"[^\"]*\">|<meta name=\"archive-api\" content=\"$ARCHIVE_API_URL\">|" /usr/share/nginx/html/index.html
