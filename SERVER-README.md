# haifa.news — Документация сервера

## Общее

| | |
|---|---|
| **Сайт** | https://haifa.news |
| **Сервер** | DigitalOcean VPS, Ubuntu |
| **Веб-сервер** | Apache 2.4.52 |
| **CMS** | WordPress (в `/var/www/html`) |
| **Python** | 3.10 |

---

## Автопостинг новостей

### Как работает

Скрипт `rss_to_wp.py` каждый час берёт новую статью из RSS-лент и публикует её на сайт через WordPress REST API.

**Расписание:** каждый час с 04:00 до 18:00 ежедневно
**Публикаций за запуск:** 1 пост

### RSS-источники

| Источник | Лента |
|---|---|
| Newsru Israel | `https://www.newsru.co.il/il/www/section/israel` |
| Cursorinfo | `https://cursorinfo.co.il/feed/` |

### Логика обработки каждой статьи

1. Проверяет кэш — если статья уже была опубликована, пропускает
2. Скачивает полный текст через **Readability** (python-readability)
3. Если Readability не справился → пробует **CSS-селекторы** популярных тем WP
4. При неудаче пробует с **мобильным User-Agent** (iPhone)
5. Затем пробует **AMP-версию** страницы (`/amp`, `/?amp=1`)
6. Крайний фолбэк — краткое описание из самого RSS
7. Очищает HTML: удаляет рекламу, виджеты, скрипты, ограничивает до 6 картинок
8. Ищет изображение: `media` в RSS → `og:image` → первая `<img>`
9. Конвертирует **webp → JPEG** через Pillow перед загрузкой в WP
10. Публикует пост в рубрику ID=8

### Файлы

| Файл | Назначение |
|---|---|
| `/root/rss_to_wp.py` | Основной скрипт автопостинга |
| `/root/.rss_to_wp.env` | Credentials (пароли, токены) |
| `/root/.rss_import_cache.json` | Кэш опубликованных статей |
| `/root/rss_to_wp.log` | Лог всех запусков |

### Настройки (в начале `rss_to_wp.py`)

```python
POSTS_PER_FEED    = 10    # сколько записей смотрим в ленте
MAX_POSTS_PER_RUN = 1     # публикуем не более N постов за запуск
CACHE_TTL_DAYS    = 90    # записи в кэше хранятся 90 дней
CATEGORY_ID       = 8     # ID рубрики в WordPress
PUBLISH_STATUS    = "publish"  # publish / draft
```

---

## Telegram-уведомления

Все уведомления идут в один чат. Credentials в `/root/.rss_to_wp.env`.

### Уведомления от автопостинга (`rss_to_wp.py`)

| Событие | Когда |
|---|---|
| ❌ Ошибка публикации | Каждый раз когда пост не вышел |
| 🚨 Критическая ошибка | 3 поста подряд не опубликованы |
| ⚠️ Лента не отвечает | RSS вернул 0 записей |
| 🔴 WordPress недоступен | WP API не отвечает перед запуском |

### Дневной дайджест (`rss_digest.py`)

Каждый день в **18:30** приходит сводка за день:
- сколько постов опубликовано
- сколько ошибок
- были ли посты без картинки или только с кратким текстом

### Уведомления от обновления сервера (`update_and_notify.sh`)

| Событие | Сообщение |
|---|---|
| Обновления установлены | ✅ Сервер обновлён + список пакетов |
| Обновлений не было | ✅ Сервер проверен |
| Ошибка apt | 🚨 Ошибка обновления + код ошибки |

---

## Обновление сервера

**Скрипт:** `/usr/local/bin/update_and_notify.sh`
**Расписание:** каждую субботу в 01:00

Запускает `apt-get update && apt-get upgrade -y`, определяет были ли изменения по выводу apt, отправляет результат в Telegram.

**Лог:** `/var/log/update_and_notify.log`

---

## Cron-задачи

```
# Автопостинг новостей: каждый час 04:00–18:00
0 4-18 * * * /usr/bin/python3 /root/rss_to_wp.py >> /root/rss_to_wp.log 2>&1

# Дневной дайджест: 18:30
30 18 * * * /usr/bin/python3 /root/rss_digest.py >> /root/rss_to_wp.log 2>&1

# Обновление сервера: каждую субботу в 01:00
0 1 * * 6 /usr/local/bin/update_and_notify.sh
```

---

## WordPress

### Тема
**covernews-pro** (активна)

### Ключевые плагины

| Плагин | Назначение |
|---|---|
| **wp-super-cache** | Кеширование страниц — отдаёт статический HTML, PHP не запускается |
| **autoptimize** | Объединяет и минифицирует JS/CSS (51→32 скриптов, 27→2 стилей) |
| **elementor** | Конструктор страниц |
| **wordpress-seo** (Yoast) | SEO, sitemap, мета-теги |
| **ewww-image-optimizer** | Оптимизация загружаемых картинок |
| **wptelegram** | Интеграция с Telegram (можно настроить автопостинг в канал) |
| **google-site-kit** | Google Analytics / Search Console |
| **mailchimp-for-wp** | Email-подписка |
| **pojo-accessibility** | Доступность сайта |
| **google-language-translator** | Перевод сайта |
| **wp-live-chat-support** | Онлайн-чат |
| **allow-mime-types** | mu-plugin: разрешает загрузку webp/avif в медиатеку |

### Производительность после оптимизации

| Метрика | До | После |
|---|---|---|
| Размер HTML | 270 КБ | 181 КБ |
| Внешних скриптов | 51 | 32 |
| Внешних стилей | 27 | 2 |

### Важные пути

| Путь | Что там |
|---|---|
| `/var/www/html` | Корень WordPress |
| `/var/www/html/wp-config.php` | Конфиг WP (DB, ключи) |
| `/var/www/html/wp-content/mu-plugins/allow-mime-types.php` | Разрешение webp/avif |
| `/var/www/html/wp-content/cache/supercache/` | Кеш страниц |
| `/var/www/html/wp-content/cache/autoptimize/` | Минифицированные JS/CSS |

---

## Credentials

Все credentials хранятся в **`/root/.rss_to_wp.env`** (права 600).

```
WP_SITE=https://haifa.news
WP_USER=admin
WP_PASS=...          # Application Password для WP REST API
TG_TOKEN=...         # Telegram Bot Token
TG_CHAT_ID=...       # Telegram Chat ID
```

Этот файл читается и `rss_to_wp.py`, и `update_and_notify.sh` — менять credentials нужно только в одном месте.

---

## Ротация логов

| Лог | Конфиг | Периодичность | Хранится |
|---|---|---|---|
| `/root/rss_to_wp.log` | `/etc/logrotate.d/rss_to_wp` | еженедельно | 8 недель |
| `/var/log/update_and_notify.log` | `/etc/logrotate.d/update_and_notify` | еженедельно | 12 недель |

---

## Быстрые команды

```bash
# Запустить постинг вручную
python3 /root/rss_to_wp.py

# Посмотреть последние записи лога
tail -50 /root/rss_to_wp.log

# Отправить дайджест вручную
python3 /root/rss_digest.py

# Сбросить кеш WordPress
wp --allow-root --path=/var/www/html cache flush

# Сбросить кеш Autoptimize
rm -rf /var/www/html/wp-content/cache/autoptimize/css/* \
       /var/www/html/wp-content/cache/autoptimize/js/*

# Сбросить кеш Super Cache
rm -rf /var/www/html/wp-content/cache/supercache/*

# Посмотреть активные плагины WP
wp --allow-root --path=/var/www/html plugin list --status=active

# Обновить все плагины WP
wp --allow-root --path=/var/www/html plugin update --all
```
