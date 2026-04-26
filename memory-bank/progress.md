# Progress

## What Works

### s3lite package (core)
- `New(path, bucket)` — открывает Badger (включая in-memory режим)
- `Set/Get/Del` — базовые операции
- `GetInfo/SetInfo` — метаданные объектов
- `List(prefix)` — итерация по ключам с группировкой по папкам
- `Count(prefix)` — количество ключей
- `IsFolder/IsFolderWithFiles/Dir` — вспомогательные методы
- `Close` — закрытие обоих Badger DB

### multy package (multi-process)
- `New(dbPath, bucket)` — first-wins: сервер или клиент
- Server mode: Unix socket listener, executeRequest для всех методов
- Client mode: gob-соединение, forward всех методов
- Auto-takeover: при падении сервера клиент перехватывает Badger
- Health check: периодическая проверка соединения
- `Close` — корректное закрытие сокета / соединения

### multy package (protocol)
- `proto.go` — gob-сериализация Request/Response
- Все 8 методов (Get, Set, Del, List, GetInfo, SetInfo, Count, Close)
- Deadline-based health check на стороне клиента

### serve package (S3 HTTP)
- HTTP сервер, совместимый с S3 API
- Готовый cmd/server/main.go

### Integration: ai project
- `tool_keyvalue.go` использует `multy.New()`
- Тип переменной: `s3lite.KeyValueStore` (интерфейс)
- Корректная сборка (go build ./... проходит)

## What's Left

### Known issues to fix
1. Client mode List() — все ключи собираются в память на сервере. Для тысяч ключей может быть проблемой.
2. Badger LOCK cleanup при SIGKILL сервера
3. Client reconnection — операции не повторяются при временных потерях сокета

### Missing features (not critical)
- Backup/Restore API
- Chunked List через сокет
- TTL/Expiration
- Batch operations
- Unix socket authentication (SO_PEERCRED)
- Prometheus metrics

### Testing
- Модульные тесты для s3lite (см. s3lite_test.go)
- Нужны интеграционные тесты для multy (server + client + takeover цикл)
- Нужны тесты для serve HTTP сервера

## Decision Log

| Date | Decision | Reason |
|------|----------|--------|
| 2026-04-26 | Изолировать `multy` как отдельный пакет, не часть s3lite | Не заставлять всех пользователей s3lite тащить Unix socket зависимости |
| 2026-04-26 | gob вместо protobuf для протокола | Нет генерации кода, достаточно для AI-сценариев |
| 2026-04-26 | Использовать `atomic.Bool` для защиты takeover | Race condition при конкурентном захвате несколькими клиентами |
| 2026-04-26 | `takingOver` — RLock для операций мультиплексирования | Предотвратить отправку запросов во время takeover |
| 2026-04-26 | Health check с 1ms deadline | Быстрая диагностика без sleep(1s) |