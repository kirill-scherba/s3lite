# System Patterns & Architecture

## Architecture Overview

```
┌──────────────────────────────────────────────────────┐
│                     Application                      │
│  (user code, ai, etc.)                               │
└──────────────────────┬───────────────────────────────┘
                       │ uses KeyValueStore interface
┌──────────────────────▼───────────────────────────────┐
│              S3LiteMulty (multy package)             │
│  ┌─────────────────┐      ┌─────────────────────────┐│
│  │   Server mode   │      │    Client mode          ││
│  │  (owns Badger)  │      │ (Unix socket forward)   ││
│  └───────┬─────────┘      └──────────┬──────────────┘│
│          │                           │               │
│  ┌───────▼─────────┐      ┌──────────▼──────────────┐│
│  │   s3lite.S3Lite │      │   clientConn            ││
│  │  (direct Badger)│      │  (net.Conn)             ││
│  └───────┬─────────┘      └──────────┬──────────────┘│
└──────────┼───────────────────────────┼───────────────┘
           │                           │
  ┌────────▼────────┐         ┌────────▼────────┐
  │   BadgerDB      │         │  Unix socket    │
  │  (database)     │         │  (/tmp/...sock) │
  └─────────────────┘         └─────────────────┘
```

## Key Design Decisions

### 1. KeyValueStore Interface (`s3lite_api.go`)
- Определяет единый контракт для всех реализаций
- `S3Lite` (прямой доступ к Badger) и `S3LiteMulty` (client/server через Unix socket) реализуют один интерфейс
- Позволяет прозрачно подменять реализацию вызовом `s3lite.New()` → `multy.New()`

### 2. Multi-Process Pattern (`multy` package)
- **First-wins**: первый процесс успешно открывает Badger и становится сервером
- **Client mode**: остальные подключаются через Unix socket
- **Auto-takeover**: если сервер умирает, клиент автоматически перехватывает Badger lock и становится сервером
- **Health check**: каждую секунду клиент проверяет живость соединения read-ом с 1ms deadline. Timeout = соединение живо. Настоящая ошибка = сервер мёртв → takeover.

### 3. Protocol (`multy/proto.go`)
- Сериализация через `encoding/gob`
- `Request{ID, Method, Key, Value, Info, Keys, Prefix}`
- `Response{ID, Value, Info, Keys, Count, Err}`
- Методы: Get, Set, Del, List, GetInfo, SetInfo, Count

### 4. Data Layout (Badger)
- На каждый bucket открывается два Badger DB:
  - `<path>/<bucket>.s3lite` — данные (key → value)
  - `<path>/<bucket>.s3lite-info` — метаданные (key → JSON ObjectInfo)
- In-memory режим: если path == `/`, открывается с `WithInMemory(true)`

### 5. List с hierarchical prefix
- List(prefix) возвращает `iter.Seq` с группировкой по папкам (символ `/`)
- Не показывает файлы внутри подпапок (flat view на один уровень)
- Использует map для дедупликации подпапок

## Component Relationships

| Package | File | Role |
|---------|------|------|
| `s3lite`| `s3lite.go` | Badger-обёртка, реализация `KeyValueStore` |
| `s3lite`| `s3lite_api.go` | `KeyValueStore` интерфейс |
| `s3lite`| `object_info.go` | `ObjectInfo` структура + JSON marshal/unmarshal |
| `multy` | `multy.go` | `S3LiteMulty` — multi-process реализация `KeyValueStore` |
| `multy` | `proto.go` | gob-протокол Request/Response |
| `multy` | `server.go` | Unix socket сервер (accept loop, execute) |
| `multy` | `client.go` | Unix socket клиент (connect, health check, takeover) |
| `serve` | `serve.go` | HTTP S3-совместимый сервер |
| `cmd/server` | `main.go` | Готовый исполняемый S3 HTTP сервер |