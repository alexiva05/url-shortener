# URL Shortener

Сервис на Go для хранения URL и коротких псевдонимов. HTTP-слой — chi, база — PostgreSQL, подключение через pgxpool.

Пока реализован один маршрут: `POST /url`. Он принимает ссылку и возвращает alias — заданный вручную или случайный из шести символов. Редирект по короткой ссылке ещё не подключён. В текущей SQL-вставке есть ошибка порядка аргументов; подробности — в [ограничениях](#ограничения).

## Запуск локально

Нужны Go 1.25.6 или новее, Docker и Docker Compose. Compose поднимает только PostgreSQL; приложение запускается отдельно.

```sh
git clone https://github.com/alexiva05/url-shortener.git
cd url-shortener
docker compose up -d postgres
cp config/local.yaml.example config/local.yaml
```

В `config/local.yaml` замени `storage_path` на строку подключения к базе из Compose:

```yaml
env: "local"
storage_path: "postgres://user:password@localhost:5432/urlshortener?sslmode=disable"
http_server:
  address: "localhost:8082"
  timeout: 4s
  idle_timeout: 60s
```

Дождись готовности PostgreSQL:

```sh
docker compose exec postgres pg_isready -U user -d urlshortener
```

Когда команда покажет `accepting connections`, запускай сервер:

```sh
CONFIG_PATH=./config/local.yaml go run ./cmd/url-shortener
```

При старте приложение само создаёт таблицу `urls` и индекс. Отдельной команды миграций нет. Сервер слушает `localhost:8082`, PostgreSQL — порт `5432`; данные базы сохраняются в Docker volume `postgres_data`.

Остановить приложение можно через `Ctrl+C`, базу — командой `docker compose down`. Volume с данными при этом сохраняется.

## API

### `POST /url`

```sh
curl -i http://localhost:8082/url \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/doc/","alias":"go-docs"}'
```

Успешный ответ:

```json
{"status":"OK","alias":"go-docs"}
```

| Поле | Обязательное | Поведение |
| --- | --- | --- |
| `url` | Да | Проверяется валидатором по правилам `required,url` |
| `alias` | Нет | Если отсутствует или пустое, генерируется строка длиной 6 символов из `A–Z`, `a–z`, `0–9` |

Пример с генерацией alias:

```sh
curl -sS http://localhost:8082/url \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/tour/"}'
```

Ответ содержит только alias, без полного адреса короткой ссылки. Маршрута `GET /{alias}` пока нет.

Ошибки возвращаются в JSON:

```json
{"status":"Error","error":"field URL is a required field"}
```

| Ситуация | `error` |
| --- | --- |
| Пустое тело или некорректный JSON | `failed to decode request` |
| Нет обязательного URL | `field URL is a required field` |
| URL не прошёл валидацию | `field URL is not a valid URL` |
| Нарушено ограничение уникальности в БД | `url already exists` |
| Другая ошибка записи | `failed to add url` |

Обработчик возвращает **HTTP 200 и при успехе, и при перечисленных ошибках**. Клиенту нужно проверять поле `status` в теле ответа.

## Конфигурация

Путь к YAML-файлу задаётся обязательной переменной `CONFIG_PATH`. Без неё или при ошибке чтения конфига приложение завершается.

| Параметр YAML | Назначение |
| --- | --- |
| `env` | `local` — текстовые логи с DEBUG; `dev` — JSON с DEBUG; `prod` — JSON с INFO |
| `storage_path` | Строка подключения к PostgreSQL, несмотря на название поля |
| `http_server.address` | Адрес HTTP-сервера |
| `http_server.timeout` | Таймаут чтения и записи HTTP-запросов |
| `http_server.idle_timeout` | Таймаут простоя keep-alive соединения |

Используй одно из трёх значений `env`: для других режимов логгер не создаётся. Файл `config/local.yaml` исключён из Git; шаблон конфигурации хранится в репозитории. Учётные данные из Compose предназначены для локального запуска.

## Как устроен код

```text
cmd/url-shortener/main.go          конфигурация, БД, middleware, маршруты
config/local.yaml.example         шаблон локального конфига
internal/config/                  загрузка YAML через cleanenv
internal/http-server/
  handlers/url/save/              разбор запроса, валидация, сохранение
  middleware/logger/              логирование HTTP-запросов через slog
  middleware/url/                 извлечение расширения из пути запроса
internal/storage/
  storage.go                      общие ошибки хранилища
  postgres/                       создание таблицы, SaveURL/GetURL/DeleteURL
lib/api/response/                 JSON-ответы и сообщения валидации
lib/logger/sl/                    атрибут ошибок для slog
lib/random/                       генерация alias и её тесты
```

Обработчик сохранения зависит от интерфейса `URLSaver` с одним методом `SaveURL`. PostgreSQL реализует этот интерфейс. Методы чтения и удаления уже есть в хранилище, но HTTP-маршруты для них не зарегистрированы.

## Проверка

```sh
go test ./...
go build ./cmd/url-shortener
```

Сейчас тесты есть только у генератора случайных строк: они проверяют длину и различие двух результатов. Проверка различия вероятностная — особенно для длины 1, где совпадение возможно с вероятностью 1/62. API и PostgreSQL тестами пока не покрыты.

При проверке на Go 1.26.1 тесты генератора прошли, но `go test ./...` завершился ошибкой встроенной проверки vet в `main.go`: `second argument to errors.As should not be *error`. В обработке результата `ListenAndServe` для сравнения с `http.ErrServerClosed` нужен `errors.Is` вместо `errors.As`.

## Ограничения

- **Перепутаны аргументы вставки в БД.** В `internal/storage/postgres/postgres.go` запрос вставляет `(alias, url)`, а `QueryRow` получает `(urlToSave, alias)`. В результате значения попадают в противоположные колонки, повторный исходный URL вызывает конфликт уникальности, а поиск по возвращённому alias не находит ожидаемую запись. Ответ `OK` не означает, что пара сохранена правильно.
- **Нет редиректа.** Сохранённый alias пока нельзя открыть как короткую ссылку.
- **Нет обработки коллизий генератора.** Повторная генерация при совпадении alias не реализована; пользовательский alias также не валидируется.
- **Ошибки API не разделены HTTP-кодами.** Для ошибок валидации, конфликта и сбоя БД обработчик использует HTTP 200.
- **Нет graceful shutdown.** Обработка сигналов и вызов `http.Server.Shutdown` пока отсутствуют.

Следующие шаги: исправить порядок SQL-аргументов, покрыть сохранение интеграционным тестом, добавить редирект и корректные HTTP-коды ошибок.
