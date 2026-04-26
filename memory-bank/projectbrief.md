# Project Brief

## Goal
S3Lite — легковесное Golang key-value хранилище, похожее на Amazon S3. Позволяет хранить и получать данные по ключу, используя BadgerDB в качестве бекенда.

## Key Requirements
- `Set(key, value)` / `Get(key)` / `Del(key)` — базовые операции
- `List(prefix)` — итерация по ключам с префиксом
- `GetInfo(key)` / `SetInfo(key, info)` — метаданные объекта (content-type, checksum, timestamps)
- Поддержка multi-process: несколько процессов на одной машине могут одновременно обращаться к одному bucket через Unix socket (пакет `multy`)
- HTTP(S) сервер по протоколу S3 (пакет `serve`)
- Использует BadgerDB v3 — встраиваемая key-value база данных

## Target Audience
Golang-разработчики, которым нужно простое встраиваемое S3-подобное хранилище.