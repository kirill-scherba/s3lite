# Technical Context

## Technology Stack

| Technology | Purpose |
|------------|---------|
| Go 1.23+   | Язык (использует `iter.Seq` из Go 1.23) |
| BadgerDB v3 | Встраиваемая key-value база данных (dgraph-io/badger) |
| encoding/gob| Сериализация для Unix socket протокола |
| net (unix)  | Unix domain sockets для multi-process (`multy`) |
| net/http    | HTTP сервер для S3-совместимости (`serve`) |
| encoding/json | Хранение ObjectInfo в Badger |
| crypto/md5  | Контрольные суммы объектов |

## Constraints & Gotchas

### BadgerDB
- **File lock**: Badger блокирует директорию на уровне файловой системы. Только один процесс может открыть один Badger DB. Это — причина создания `multy` пакета.
- **Resource usage**: Badger плохо закрывается при `SIGKILL`, может потребоваться ручная чистка `LOCK` файла после падения.
- **Memory**: Badger по умолчанию использует mmap, может потреблять много памяти при больших базах.

### Unix Socket (multy)
- **Сокет лежит в `/tmp`**: формат имени `/tmp/s3lite-<bucket>.sock`
- **Stale socket cleanup**: `startServer()` удаляет старый сокет через `os.Remove` перед `net.Listen`
- **Auto-takeover race condition**: защищён `atomic.Bool` (`takingOver`), чтобы избежать конкурентного захвата несколькими клиентами

### gob encoding
- **Stream protocol**: каждый Request/Response — отдельный gob.Encode/Decode
- **Thread safety**: gob не thread-safe, но в текущей архитектуре каждый клиент в своей горутине с последовательным чтением/записью

### Go version
- `iter.Seq` требует Go 1.23+
- `strings.CutSuffix` (используется в s3lite) — Go 1.20+

## Dependencies

### s3lite (go.mod)
```
module github.com/kirill-scherba/s3lite
go 1.26.1
require github.com/dgraph-io/badger/v3 v3.2103.5
```
(Информация из `s3lite/go.mod`)

### ai (go.mod)
```
module github.com/kirill-scherba/ai
go 1.24.4
require github.com/kirill-scherba/s3lite v0.0.0-...
```
(Актуальная версия будет обновлена при `go mod tidy`)

## Development Setup

### Сборка пакета s3lite
```bash
cd ~/go/src/github.com/kirill-scherba/s3lite
go build ./...
```

### Сборка пакета multy
```bash
cd ~/go/src/github.com/kirill-scherba/s3lite
go build ./multy/...
```

### Сборка проекта ai
```bash
cd ~/go/src/github.com/kirill-scherba/ai
go build ./...
```

## Testing

- Модульные тесты в `s3lite/*_test.go` (если есть)
- Интеграционные тесты для multy: запуск сервера + клиент + проверка операций

## Production Considerations

- **File descriptors**: Unix socket на каждый bucket = один listener + N connections
- **Backup**: Badger DB — это директория с SST файлами. Бекап через копирование (лучше через Badger Stream API, но пока не реализовано)
- **Monitoring**: Логирование в `log.Printf` на стороне сервера при ошибках записи ответа