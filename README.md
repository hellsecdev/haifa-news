# Haifa.News

Русскоязычная городская лента о Хайфе и севере Израиля. Статический сайт для GitHub Pages — без PostgreSQL и без сервера.

**Сайт:** https://haifa.news

## Быстрый старт

```bash
python3 scripts/build_static.py --base-url https://haifa.news
python3 -m http.server 8080   # http://localhost:8080
```

Подробнее: [STATIC.md](STATIC.md)

## GitHub Pages + custom domain

1. Соберите сайт (команда выше).
2. Файл [`CNAME`](CNAME) содержит `haifa.news` — не удаляйте его.
3. На GitHub: **Settings → Pages → Custom domain** → `haifa.news`.
4. **Source:** Deploy from branch → `main` → `/ (root)`.

## Структура

| Путь | Назначение |
|---|---|
| `index.html`, `article/`, `category/` | Сгенерированный статический сайт |
| `uploads/` | Медиафайлы статей |
| `assets/` | CSS |
| `export/wp-export.json` | Исходные данные для сборки |
| `scripts/build_static.py` | Генератор статики |
| `backend/` | Go API (только для прод-сервера) |

## Обновление новостей

1. Обновите `export/wp-export.json`.
2. `python3 scripts/build_static.py --base-url https://haifa.news`
3. Commit + push.
