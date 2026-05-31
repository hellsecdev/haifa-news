# Haifa.News — static build

Статический сайт для **GitHub Pages** с custom domain **https://haifa.news**.

## Сборка

```bash
python3 scripts/build_static.py --base-url https://haifa.news
```

Для GitHub Pages без custom domain (подпапка):

```bash
python3 scripts/build_static.py --base-url https://hellsecdev.github.io/haifa-news
```

## Публикация

1. Соберите сайт.
2. Убедитесь, что [`CNAME`](CNAME) содержит `haifa.news`.
3. Push в `main`.
4. GitHub → **Settings → Pages** → Custom domain: `haifa.news`, branch `main`, folder `/ (root)`.

## DNS (если ещё не настроено)

| Тип | Имя | Значение |
|---|---|---|
| A | `@` | `185.199.108.153` … `185.199.111.153` (4 адреса GitHub Pages) |
| CNAME | `www` | `hellsecdev.github.io` |

Или один ALIAS/ANAME на `hellsecdev.github.io` — зависит от регистратора.

## Обновление контента

1. Обновите `export/wp-export.json`.
2. `python3 scripts/build_static.py --base-url https://haifa.news`
3. Commit + push.
