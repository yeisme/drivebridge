#!/usr/bin/env python3
"""Capture a stable, read-only OpenDesign reference in the evidence directory."""
import argparse
import hashlib
import json
import os
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urljoin, urlsplit
from urllib.request import HTTPRedirectHandler, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError('Redirected prototype assets are not accepted.')


class EntryAssets(HTMLParser):
    def __init__(self):
        super().__init__()
        self.paths = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        value = attrs.get('src') if tag == 'script' else attrs.get('href') if tag == 'link' and attrs.get('rel') == 'stylesheet' else None
        if value:
            self.paths.append(value)


def references(entry, html):
    parser = EntryAssets()
    parser.feed(html)
    root = urljoin(entry, './')
    urls = [entry]
    for value in parser.paths:
        url = urljoin(entry, value)
        parsed = urlsplit(url)
        if parsed.username or parsed.password or parsed.fragment or not url.startswith(root):
            raise ValueError('Reference must remain within the OpenDesign project.')
        if url not in urls:
            urls.append(url)
    return urls


def capture(entry, output, fetch):
    first = fetch(entry)
    urls = references(entry, first.decode('utf-8'))
    bodies = {entry: first}
    for url in urls[1:]:
        bodies[url] = fetch(url)
    # Query strings are cache keys, not immutable versions. Re-read all inputs.
    for url in urls:
        if fetch(url) != bodies[url]:
            raise ValueError('Prototype changed during capture; no snapshot was accepted.')
    output.mkdir(parents=True, exist_ok=True)
    assets = []
    for index, url in enumerate(urls):
        body = bodies[url]
        name = f'{index:03d}-{Path(urlsplit(url).path).name}'
        (output / name).write_bytes(body)
        assets.append({'url': url, 'file': name, 'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()})
    manifest = {'entry': entry, 'scope': 'entry-and-direct-scripts-styles', 'stable_double_read': True,
                'business_verified': False, 'assets': assets}
    (output / 'snapshot-manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--entry', required=True)
    args = parser.parse_args()
    entry = urlsplit(args.entry)
    if entry.scheme not in ('http', 'https') or entry.username or entry.password or entry.query or entry.fragment:
        parser.error('Use an HTTP(S) project entry without credentials, query or fragment.')
    evidence = os.environ.get('DRIVEBRIDGE_EVIDENCE_ARTIFACTS')
    if not evidence:
        parser.error('Run through scripts/check.py to preserve owner evidence.')

    def fetch(url):
        if urlsplit(url).scheme not in ('http', 'https'):
            raise ValueError('Only HTTP(S) reference reads are allowed.')
        with build_opener(NoRedirect()).open(url, timeout=20) as response:
            if response.geturl() != url:
                raise ValueError('Redirected prototype assets are not accepted.')
            body = response.read(16 * 1024 * 1024 + 1)
            if len(body) > 16 * 1024 * 1024:
                raise ValueError('Prototype asset exceeds the 16 MiB capture limit.')
            return body

    manifest = capture(args.entry, Path(evidence) / 'prototype', fetch)
    print(f"Captured {len(manifest['assets'])} stable reference resources; not a business verification.")


if __name__ == '__main__':
    main()
