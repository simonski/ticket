#!/usr/bin/env python3
"""Static server mirroring the Go server's asset overlay for browser tests.

Serves web/default/ first, falling back to web/shared/, with an SPA fallback
to default/index.html — the same resolution order as web.SiteFS in static.go.
"""
import http.server
import os
import sys

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 4174
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "web"))


class OverlayHandler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self, path):
        rel = path.split("?", 1)[0].split("#", 1)[0].lstrip("/")
        if rel == "":
            rel = "index.html"
        for base in ("default", "shared"):
            candidate = os.path.join(ROOT, base, *rel.split("/"))
            if os.path.isfile(candidate):
                return candidate
        # SPA fallback: unknown routes serve the app shell.
        return os.path.join(ROOT, "default", "index.html")

    def log_message(self, fmt, *args):
        pass


http.server.ThreadingHTTPServer(("127.0.0.1", PORT), OverlayHandler).serve_forever()
