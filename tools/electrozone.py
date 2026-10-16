"""Search Electrozone (electrozone.com.mx) products from the terminal.

    uv run tools/electrozone.py JST "PH2.0" XL4015       # one search per word
    uv run tools/electrozone.py --csv out.csv caiman      # also save a CSV
    uv run tools/electrozone.py --all --csv catalog.csv   # crawl the whole catalog (slow)

Each result: code, price, stock in the Tecnológico and Torres stores, description.
Standard library only.
"""
from __future__ import annotations

import argparse
import csv
import html
import re
import sys
import time
import urllib.parse
import urllib.request

BASE = "https://www.electrozone.com.mx/ezone2022/"
UA = {"User-Agent": "Mozilla/5.0 (bb8 parts search)"}


def fetch(path: str, data: dict | None = None) -> str:
    url = BASE + path.replace(" ", "%20")
    body = urllib.parse.urlencode(data).encode() if data else None
    req = urllib.request.Request(url, body, UA)
    with urllib.request.urlopen(req, timeout=30) as r:
        return r.read().decode("latin-1")


def parse_products(page: str) -> list[dict]:
    """Products appear as text lines: code, description, $price, 'Tec: n Torres: n'."""
    text = re.sub(r"<script.*?</script>|<style.*?</style>", "", page, flags=re.S)
    text = html.unescape(re.sub(r"<[^>]+>", "\n", text))
    lines = [ln.strip() for ln in text.split("\n") if ln.strip()]
    out = []
    for i, ln in enumerate(lines):
        m = re.match(r"Tec:\s*(-?\d+)\s*Torres:\s*(-?\d+)", ln)
        if not m or i < 3 or not lines[i - 1].startswith("$"):
            continue
        price = float(lines[i - 1].lstrip("$").replace(",", ""))
        out.append({"code": lines[i - 3], "description": lines[i - 2], "price": price,
                    "tec": int(m[1]), "torres": int(m[2])})
    return out


def search(word: str) -> list[dict]:
    return parse_products(fetch("lista_articulos.php", {
        "cod_articulo": "", "desc1_articulo": word, "desc2_articulo": "",
        "buscar": "Buscar Articulos"}))


def crawl_all() -> list[dict]:
    """Families → groups → product lists. Takes a few minutes; be polite."""
    links = lambda page, kind: sorted(set(re.findall(rf'href="({kind}\.php\?[^"]*)"', page)))  # noqa: E731
    seen, out = set(), []
    for fam in links(fetch("index.php"), "lista_grupos"):
        for grp in links(fetch(html.unescape(fam)), "lista_articulos_grupo"):
            print(f"  {urllib.parse.unquote(grp)[:90]}", file=sys.stderr)
            for p in parse_products(fetch(html.unescape(grp))):
                if p["code"] not in seen:
                    seen.add(p["code"])
                    out.append(p)
            time.sleep(0.3)
    return out


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("words", nargs="*", help="words to search (one search each)")
    ap.add_argument("--all", action="store_true", help="crawl the whole catalog")
    ap.add_argument("--in-stock", action="store_true", help="only items with stock in some store")
    ap.add_argument("--csv", help="also write results to this CSV file")
    a = ap.parse_args()
    if not a.words and not a.all:
        ap.error("give words to search or --all")

    rows = []
    if a.all:
        rows = crawl_all()
    for w in a.words:
        found = search(w)
        rows += [{**p, "search": w} for p in found]
        if not a.csv:
            print(f"\n== {w}: {len(found)} result(s)")
            for p in found:
                if a.in_stock and p["tec"] <= 0 and p["torres"] <= 0:
                    continue
                print(f"{p['code']:<16} ${p['price']:>8.2f}  Tec {p['tec']:>4}  Torres {p['torres']:>4}  {p['description'][:110]}")
    if a.in_stock:
        rows = [r for r in rows if r["tec"] > 0 or r["torres"] > 0]
    if a.csv:
        with open(a.csv, "w", newline="", encoding="utf-8") as f:
            w = csv.DictWriter(f, ["search", "code", "price", "tec", "torres", "description"], extrasaction="ignore")
            w.writeheader()
            w.writerows(rows)
        print(f"{len(rows)} rows → {a.csv}")


if __name__ == "__main__":
    main()
