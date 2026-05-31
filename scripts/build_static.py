#!/usr/bin/env python3
"""Generate a static Haifa.News site for GitHub Pages (no database required)."""

from __future__ import annotations

import argparse
import html
import json
import shutil
import sys
from datetime import datetime
from pathlib import Path
from urllib.parse import quote, urlparse

ROOT = Path(__file__).resolve().parent.parent
EXPORT = ROOT / "export" / "wp-export.json"
ASSET_VER = "20260525v8"
HOME_LIMIT = 12
CATEGORY_PAGE_SIZE = 60
BASE_PATH = ""


def set_base_path(base_url: str, base_path: str | None) -> None:
    global BASE_PATH
    if base_path is not None:
        BASE_PATH = base_path.rstrip("/")
        if BASE_PATH and not BASE_PATH.startswith("/"):
            BASE_PATH = f"/{BASE_PATH}"
        return
    path = urlparse(base_url).path.strip("/")
    BASE_PATH = f"/{path}" if path else ""


def u(path: str) -> str:
    if not path.startswith("/"):
        path = f"/{path}"
    return f"{BASE_PATH}{path}" if BASE_PATH else path


def esc(text: str) -> str:
    return html.escape(html.unescape(text or ""), quote=True)


def fix_url(url: str) -> str:
    if url.startswith("/"):
        return u(url)
    return url


def rewrite_html(content: str) -> str:
    if not content:
        return ""
    for attr in ("src", "href"):
        for root in ("/uploads/", "/assets/"):
            content = content.replace(f'{attr}="{root}', f'{attr}="{u(root)}')
            content = content.replace(f"{attr}='{root}", f"{attr}='{u(root)}")
    return content


def fmt_date(iso: str) -> str:
    try:
        dt = datetime.fromisoformat(iso.replace("Z", "+00:00"))
        return dt.strftime("%d.%m.%Y")
    except ValueError:
        return ""


def fmt_iso(iso: str) -> str:
    try:
        dt = datetime.fromisoformat(iso.replace("Z", "+00:00"))
        return dt.strftime("%Y-%m-%dT%H:%M:%SZ")
    except ValueError:
        return iso


def article_href(slug: str) -> str:
    return u(f"/article/{quote(slug, safe='')}")


def category_href(slug: str) -> str:
    return u(f"/category/{quote(slug, safe='')}")


def head(title: str, desc: str, canonical: str) -> str:
    return f"""<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{esc(title)}</title>
<meta name="description" content="{esc(desc)}">
<meta name="robots" content="index, follow, max-image-preview:large">
<link rel="canonical" href="{esc(canonical)}">
<link rel="stylesheet" href="{u('/assets/index.css')}?v={ASSET_VER}">
</head>
<body>
<div id="app">"""


def nav() -> str:
    return f"""
<div class="ticker"><span><b>LIVE</b> Haifa.News · Хайфа · Север Израиля · город · безопасность · общество</span></div>
<header class="head">
<div class="wrap bar">
<a class="brand" href="{u('/')}"><span>H</span><strong>Haifa.News</strong><small>independent city desk</small></a>
<nav>
<a href="{u('/')}">Главная</a>
<a href="{u('/category/новости-израиля')}">Израиль</a>
<a href="{u('/category/новости-хайфы')}">Хайфа</a>
<a href="{u('/about')}">Редакция</a>
</nav>
<div class="search">Поиск</div>
</div>
</header>"""


def footer() -> str:
    return """
<footer><div class="wrap">Haifa.News · Новости Хайфы и севера Израиля</div></footer>
</div>
</body>
</html>"""


def post_card(post: dict) -> str:
    img = post.get("featured_image") or ""
    title = esc(post["title"])
    excerpt = esc(post.get("excerpt") or "")
    cat = post.get("category") or {}
    cat_name = esc(cat.get("name") or "")
    date = fmt_date(post.get("published_at") or "")
    href = article_href(post["slug"])

    if img:
        media = f'<img src="{esc(img)}" alt="{title}">'
    else:
        media = '<div class="ph">HAIFA.NEWS<span>חיפה</span></div>'

    return f"""<a class="card" href="{href}">
{media}
<div>
<p class="meta">{cat_name} · {date}</p>
<h3>{title}</h3>
<p>{excerpt}</p>
<b>читать →</b>
</div>
</a>"""


def aside_categories(categories: list[dict]) -> str:
    items = []
    for cat in categories:
        href = category_href(cat["slug"])
        items.append(
            f'<a href="{href}">{esc(cat["name"])} <span>{cat.get("count", 0)}</span></a>'
        )
    return f"""<aside>
<h3>Рубрики</h3>
{"".join(items)}
</aside>"""


def load_data() -> tuple[list[dict], list[dict]]:
    with EXPORT.open(encoding="utf-8") as f:
        data = json.load(f)

    categories = []
    for raw in data["categories"]:
        categories.append(
            {
                "wp_id": raw["wp_id"],
                "name": raw["name"],
                "slug": raw["slug"],
                "count": raw.get("count", 0),
            }
        )

    cat_by_wp = {c["wp_id"]: c for c in categories}
    posts = []
    for raw in data["posts"]:
        if raw.get("status") != "publish":
            continue
        cat = cat_by_wp.get(raw.get("category_wp_id"))
        posts.append(
            {
                "title": raw["title"],
                "slug": raw["slug"],
                "excerpt": raw.get("excerpt") or "",
                "content": rewrite_html(raw.get("content") or ""),
                "featured_image": fix_url(raw.get("featured_image") or ""),
                "published_at": raw.get("published_at") or "",
                "updated_at": raw.get("updated_at") or raw.get("published_at") or "",
                "category": cat,
            }
        )

    posts.sort(key=lambda p: p["published_at"], reverse=True)

    # Recompute counts from published posts.
    counts: dict[str, int] = {}
    for post in posts:
        cat = post.get("category")
        if cat:
            counts[cat["slug"]] = counts.get(cat["slug"], 0) + 1
    for cat in categories:
        cat["count"] = counts.get(cat["slug"], 0)

    categories.sort(key=lambda c: (-c["count"], c["name"]))
    return categories, posts


def write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def build_home(base_url: str, categories: list[dict], posts: list[dict], out: Path) -> None:
    cards = "".join(post_card(p) for p in posts[:HOME_LIMIT])
    page = (
        head(
            "Haifa.News — новости Хайфы",
            "Новости Хайфы, Израиля и севера: город, безопасность, бизнес, культура.",
            f"{base_url}/",
        )
        + nav()
        + """
<main>
<section class="wrap hero">
<div class="hero-copy">
<p class="kicker">Новости Хайфы</p>
<h1>Главные события Хайфы и севера Израиля.</h1>
<p>Городские новости, безопасность, транспорт, общество, бизнес и культура — всё важное для жителей Хайфы на русском языке.</p>
</div>
<div class="radar"><span>חיפה</span><i></i><em>HAIFA NEWSROOM</em></div>
</section>
<section class="wrap layout">
<div>
<h2>Лента новостей</h2>
<div class="grid">"""
        + cards
        + """
</div>
</div>
<aside>
<h3>Редакция</h3>
<p>Сообщить новость или связаться с редакцией.</p>
<a href="mailto:newsroom@haifa.news">newsroom@haifa.news</a>
</aside>
</section>
</main>"""
        + footer()
    )
    write(out / "index.html", page)


def build_about(base_url: str, categories: list[dict], out: Path) -> None:
    page = (
        head(
            "Редакция — Haifa.News",
            "Редакция Haifa.News: новости Хайфы и севера Израиля на русском языке.",
            f"{base_url}/about",
        )
        + nav()
        + """
<main class="wrap article-page">
<article class="article">
<p class="kicker">Haifa.News</p>
<h1>Редакция</h1>
<p class="lead">Haifa.News — русскоязычная городская лента о Хайфе и севере Израиля: важные новости, безопасность, транспорт, общество, бизнес и культура.</p>
<div class="body">
<p>Мы собираем и публикуем материалы, которые помогают жителям Хайфы быстро понимать, что происходит в городе и регионе.</p>
<p>Сообщить новость, прислать уточнение или связаться с редакцией можно по адресу: <a href="mailto:newsroom@haifa.news">newsroom@haifa.news</a>.</p>
</div>
</article>
"""
        + aside_categories(categories)
        + """
</main>"""
        + footer()
    )
    write(out / "about" / "index.html", page)


def build_article(base_url: str, categories: list[dict], post: dict, out: Path) -> None:
    cat = post.get("category") or {}
    cat_name = esc(cat.get("name") or "Haifa.News")
    title = esc(post["title"])
    excerpt = esc(post.get("excerpt") or "")
    canonical = f"{base_url}{article_href(post['slug'])}"
    img = post.get("featured_image") or ""
    img_html = (
        f'<img class="hero-img" src="{esc(img)}" alt="{title}">' if img else ""
    )
    schema = json.dumps(
        {
            "@context": "https://schema.org",
            "@type": "NewsArticle",
            "headline": post["title"],
            "description": post.get("excerpt") or "",
            "datePublished": fmt_iso(post.get("published_at") or ""),
            "dateModified": fmt_iso(post.get("updated_at") or ""),
            "mainEntityOfPage": canonical,
            "publisher": {"@type": "Organization", "name": "Haifa.News"},
        },
        ensure_ascii=False,
    )

    page = (
        head(f"{post['title']} — Haifa.News", post.get("excerpt") or "", canonical)
        + nav()
        + f"""
<main class="wrap article-page">
<article class="article">
<p class="kicker">{cat_name} · {fmt_date(post.get("published_at") or "")}</p>
<h1>{title}</h1>
<p class="lead">{excerpt}</p>
{img_html}
<script type="application/ld+json">{schema}</script>
<div class="body">{post.get("content") or ""}</div>
</article>
"""
        + aside_categories(categories)
        + f"""
<aside><a href="{u('/')}">← Главная</a></aside>
</main>"""
        + footer()
    )
    write(out / "article" / post["slug"] / "index.html", page)


def build_category(
    base_url: str,
    categories: list[dict],
    cat: dict,
    posts: list[dict],
    out: Path,
) -> None:
    slug = cat["slug"]
    cat_posts = [p for p in posts if (p.get("category") or {}).get("slug") == slug]
    total_pages = max(1, (len(cat_posts) + CATEGORY_PAGE_SIZE - 1) // CATEGORY_PAGE_SIZE)

    for page_num in range(1, total_pages + 1):
        start = (page_num - 1) * CATEGORY_PAGE_SIZE
        chunk = cat_posts[start : start + CATEGORY_PAGE_SIZE]
        cards = "".join(post_card(p) for p in chunk)

        if page_num == 1:
            canonical = f"{base_url}{category_href(slug)}"
            path = out / "category" / slug / "index.html"
        else:
            canonical = f"{base_url}{category_href(slug)}page/{page_num}/"
            path = out / "category" / slug / "page" / str(page_num) / "index.html"

        pager = ""
        if total_pages > 1:
            links = []
            if page_num > 1:
                prev = (
                    category_href(slug)
                    if page_num == 2
                    else f"{category_href(slug)}page/{page_num - 1}/"
                )
                links.append(f'<a href="{prev}">← Назад</a>')
            links.append(f"<span>Страница {page_num} из {total_pages}</span>")
            if page_num < total_pages:
                links.append(
                    f'<a href="{category_href(slug)}page/{page_num + 1}/">Далее →</a>'
                )
            pager = f'<div class="hero-actions">{"".join(links)}</div>'

        page = (
            head(
                f"{cat['name']} — Haifa.News",
                f"Новости рубрики «{cat['name']}» на Haifa.News.",
                canonical,
            )
            + nav()
            + f"""
<main>
<section class="wrap hero">
<div class="hero-copy">
<p class="kicker">Рубрика</p>
<h1>{esc(cat["name"])}</h1>
<p>Материалы рубрики «{esc(cat["name"])}» — {len(cat_posts)} публикаций.</p>
{pager}
</div>
</section>
<section class="wrap layout">
<div>
<h2>Лента</h2>
<div class="grid">{cards}</div>
{pager}
</div>
"""
            + aside_categories(categories)
            + """
</section>
</main>"""
            + footer()
        )
        write(path, page)


def build_sitemap(base_url: str, categories: list[dict], posts: list[dict], out: Path) -> None:
    lines = [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
        f"<url><loc>{base_url}/</loc></url>",
        f"<url><loc>{base_url}/about</loc></url>",
    ]
    for cat in categories:
        if cat["count"] <= 0:
            continue
        lines.append(f"<url><loc>{base_url}{category_href(cat['slug'])}</loc></url>")
        pages = (cat["count"] + CATEGORY_PAGE_SIZE - 1) // CATEGORY_PAGE_SIZE
        for n in range(2, pages + 1):
            lines.append(
                f"<url><loc>{base_url}{category_href(cat['slug'])}page/{n}/</loc></url>"
            )
    for post in posts:
        lastmod = fmt_date(post.get("updated_at") or post.get("published_at") or "")
        iso = ""
        if lastmod:
            try:
                iso = datetime.strptime(lastmod, "%d.%m.%Y").strftime("%Y-%m-%d")
            except ValueError:
                iso = ""
        loc = f"{base_url}{article_href(post['slug'])}"
        if iso:
            lines.append(f"<url><loc>{loc}</loc><lastmod>{iso}</lastmod></url>")
        else:
            lines.append(f"<url><loc>{loc}</loc></url>")
    lines.append("</urlset>")
    write(out / "sitemap.xml", "\n".join(lines))


def build_robots(base_url: str, out: Path) -> None:
    write(
        out / "robots.txt",
        f"User-agent: *\nAllow: /\nSitemap: {base_url}/sitemap.xml\n",
    )


def copy_assets(out: Path) -> None:
    src = ROOT / "dist" / "assets"
    dst = out / "assets"
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst)


def ensure_uploads(out: Path, copy: bool) -> None:
    source = ROOT / "uploads"
    target = out / "uploads"
    if not source.exists():
        print("  warning: uploads/ not found", file=sys.stderr)
        return
    if out.resolve() == ROOT.resolve() and target.exists() and not target.is_symlink():
        return
    if target.exists() or target.is_symlink():
        if target.is_symlink() or target.is_file():
            target.unlink()
        else:
            shutil.rmtree(target)
    if copy:
        shutil.copytree(source, target)
    else:
        target.symlink_to(source.resolve(), target_is_directory=True)


def build_api_json(categories: list[dict], posts: list[dict], out: Path) -> None:
    api = out / "api"
    api.mkdir(parents=True, exist_ok=True)

    cat_json = [
        {"id": i + 1, "name": c["name"], "slug": c["slug"], "count": c["count"]}
        for i, c in enumerate(categories)
    ]
    write(api / "categories.json", json.dumps(cat_json, ensure_ascii=False, indent=2))

    meta_posts = []
    for p in posts:
        cat = p.get("category") or {}
        meta_posts.append(
            {
                "title": p["title"],
                "slug": p["slug"],
                "excerpt": p.get("excerpt") or "",
                "featured_image": p.get("featured_image") or "",
                "published_at": p.get("published_at") or "",
                "category": {
                    "name": cat.get("name"),
                    "slug": cat.get("slug"),
                }
                if cat
                else None,
            }
        )
    write(api / "posts.json", json.dumps(meta_posts, ensure_ascii=False))


def main() -> int:
    parser = argparse.ArgumentParser(description="Build static Haifa.News site")
    parser.add_argument(
        "--out",
        type=Path,
        default=ROOT,
        help="Output directory (default: repo root for GitHub Pages)",
    )
    parser.add_argument(
        "--base-url",
        default="https://haifa.news",
        help="Canonical base URL (no trailing slash)",
    )
    parser.add_argument(
        "--copy-uploads",
        action="store_true",
        help="Copy uploads/ into output instead of symlink (for CI)",
    )
    parser.add_argument(
        "--base-path",
        default=None,
        help="URL prefix for GitHub project pages, e.g. /haifa-news (auto from --base-url if omitted)",
    )
    args = parser.parse_args()
    base_url = args.base_url.rstrip("/")
    out: Path = args.out.resolve()
    set_base_path(base_url, args.base_path)
    if BASE_PATH:
        print(f"  base path: {BASE_PATH}")

    if not EXPORT.exists():
        print(f"Missing export file: {EXPORT}", file=sys.stderr)
        return 1

    print("Loading export data…")
    categories, posts = load_data()
    print(f"  {len(categories)} categories, {len(posts)} published posts")

    print(f"Building static site → {out}")
    build_home(base_url, categories, posts, out)
    build_about(base_url, categories, out)

    print("  articles…")
    for i, post in enumerate(posts, 1):
        build_article(base_url, categories, post, out)
        if i % 500 == 0:
            print(f"    {i}/{len(posts)}")

    print("  categories…")
    for cat in categories:
        if cat["count"] > 0:
            build_category(base_url, categories, cat, posts, out)

    build_sitemap(base_url, categories, posts, out)
    build_robots(base_url, out)
    build_api_json(categories, posts, out)
    copy_assets(out)

    if args.copy_uploads:
        print("  copying uploads/ (this may take a while)…")
    ensure_uploads(out, copy=args.copy_uploads)

    write(out / ".nojekyll", "")
    print("Done.")
    print(f"  Pages: 1 home + 1 about + {len(posts)} articles + category pages")
    print(f"  Open locally: python3 -m http.server --directory {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
