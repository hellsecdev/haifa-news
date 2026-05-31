# Haifa.News

Русскоязычная городская лента о Хайфе и севере Израиля. Статический сайт для GitHub Pages — без PostgreSQL и без сервера.

## Быстрый старт

```bash
python3 scripts/build_static.py --base-url https://hellsecdev.github.io/haifa-news
python3 -m http.server 8080   # http://localhost:8080/haifa-news/
```

Подробнее: [STATIC.md](STATIC.md)

## GitHub Pages (вариант A — из корня)

1. Закоммитьте и запушьте репозиторий в GitHub.
2. На GitHub: **Settings → Pages**.
3. **Source:** Deploy from a branch.
4. **Branch:** `main` → **Folder:** `/ (root)` → **Save**.

Сайт будет доступен по адресу:

**https://hellsecdev.github.io/haifa-news/**

Если позже подключите домен `haifa.news`, добавьте его в Settings → Pages → Custom domain и пересоберите с `--base-url https://haifa.news`.

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
3. Закоммитьте изменения и push.
