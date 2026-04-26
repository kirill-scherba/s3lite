# Active Context

## Current Development State

Проект полностью реализован:

- [x] `s3lite.S3Lite` — BadgerDB обёртка, реализует `KeyValueStore`
- [x] `s3lite.KeyValueStore` interface — контракт для всех реализаций
- [x] `s3lite.ObjectInfo` — метаданные объекта
- [x] `multy.S3LiteMulty` — multi-process реализация (server/client + auto-takeover)
- [x] `multy` server — Unix socket listener с gob-протоколом
- [x] `multy` client — connect, health check, takeover
- [x] `serve` — HTTP S3-совместимый сервер
- [x] `cmd/server` — исполняемый S3 сервер

## Integration with `ai` project

- `ai/tool_keyvalue.go` использует `multy.New()` вместо `s3lite.New()`
- Тип хранилища изменён с `*s3lite.S3Lite` на `s3lite.KeyValueStore`
- Все методы хранилища работают через интерфейс

## Active Decisions

### 1. Socket path location
Сокет лежит в `/tmp/s3lite-<bucket>.sock`. Альтернативы:
- `XDG_RUNTIME_DIR` — более правильное, но не везде доступно
- `/var/run` — требует прав root
- Пока `/tmp` достаточно для серверных сценариев

### 2. gob vs protobuf vs JSON
- gob — встроенный в Go, не требует генерации кода
- Protobuf был бы быстрее/компактнее, но требует отдельной генерации
- Для AI-сценариев с маленькими данными gob оптимален

### 3. Health check strategy
- Клиент проверяет соединение каждые 1000ms
- Устанавливает deadline на 1ms для read — либо таймаут (соединение живо), либо настоящая ошибка
- `io.EOF` или закрытие сокета = сервер умер → takeover

## Known Issues

1. **Badger LOCK cleanup**: если сервер упал с `SIGKILL`, клиент может получить stale LOCK и fail при takeover. Ручное удаление `LOCK`-файла или ожидание перезапуска.
2. **Client reconnection**: если клиент теряет соединение до завершения takeover, операция может упасть с ошибкой. Текущий код возвращает ошибку пользователю, а не повторяет.
3. **List() in client mode**: `List` через сокет собирает все ключи в слайс на сервере и отправляет целиком. Для больших списков это может жрать память. Итеративная передача (chunked) пока не реализована.

## What's Not Yet Implemented

- [ ] **Backup/Restore API** — Badger Stream-based backup
- [ ] **Client reconnection** — авто-реконнект после временных потерь сокета
- [ ] **Chunked List** — итеративная передача ключей через сокет
- [ ] **TTL / Expiration** — время жизни ключей (Badger поддерживает TTL)
- [ ] **Batch operations** — bulk Set/Del через сокет
- [ ] **Authentication** — авторизация клиентов на Unix socket (SO_PEERCRED)
- [ ] **Metrics** — Prometheus метрики для serve сервера
