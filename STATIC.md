# Haifa.News — static build

Статический сайт для **GitHub Pages из корня репозитория** (без PostgreSQL, без Go-сервера).

## Сборка

```bash
python3 scripts/build_static.py --base-url https://hellsecdev.github.io/haifa-news
```

Скрипт автоматически добавит префикс `/haifa-news` ко всем ссылкам, CSS и картинкам.

Для custom domain:

```bash
python3 scripts/build_static.py --base-url https://haifa.news
```

Результат пишется в корень репозитория: `index.html`, `about/`, `article/`, `category/`, `assets/`, `api/`, `sitemap.xml`, `robots.txt`, `.nojekyll`.

Локальный просмотр:

```bash
python3 -m http.server 8080
```

## Публикация на GitHub Pages

1. Соберите сайт (команда выше).
2. Закоммитьте и запушьте всё в `main` (включая `uploads/` ~1 GB).
3. GitHub → **Settings → Pages**:
   - Source: **Deploy from a branch**
   - Branch: **main**
   - Folder: **/ (root)**
4. Через 1–3 минуты сайт откроется: https://hellsecdev.github.io/haifa-news/

### Custom domain (опционально)

Если привяжете `haifa.news` в Settings → Pages, пересоберите с `--base-url https://haifa.news`.

## Обновление контента

1. Обновите `export/wp-export.json`.
2. Запустите `scripts/build_static.py`.
3. Commit + push.

## Ограничения

- `/admin` не работает на статике.
- Поиск — через навигацию по рубрикам и HTML-страницам.
