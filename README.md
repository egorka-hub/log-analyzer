# Log Processor

## Описание

Консольный инструмент на Go для анализа логов распределённых систем.

Когда запрос проходит через несколько сервисов, его следы разбросаны по разным лог-файлам. Log Processor собирает записи из всех файлов каталога, связывает их по `request_id`, находит запросы, завершившиеся с ошибкой, восстанавливает хронологию каждого из них и сохраняет результат в JSON-отчёт.

## Возможности

- **Сопоставление записей по `request_id`**: записи разных сервисов группируются в один запрос
- **Обнаружение ошибок**: запрос считается проблемным, если в нём есть запись уровня `ERROR` или `WARN`
- **Хронология сбоя**: первая ошибка по запросу и полный таймлайн, отсортированный по времени
- **Конкурентная обработка**: файлы читаются параллельно пулом воркеров (горутины + каналы)
- **Graceful shutdown**: по `Ctrl+C` воркеры дочитывают текущие файлы, а частичные результаты сохраняются в отчёт
- **JSON-отчёты**: итог анализа записывается в файл
- **Устойчивость к мусору**: битые строки и неизвестные уровни логирования пропускаются с предупреждением в stderr

## Требования

- Go 1.26 или новее

## Установка и запуск

```bash
git clone https://github.com/egorka-hub/log-analyzer.git
cd log-analyzer
go build -o log-processor .
./log-processor --input-dir /var/log/services/
```

Без сборки бинарника:

```bash
go run . --input-dir /var/log/services/
```

### Флаги

| Флаг            | По умолчанию   | Описание                                         |
|-----------------|----------------|--------------------------------------------------|
| `--input-dir`   | `.`            | Каталог с файлами `*.log` (обходится рекурсивно) |
| `--output-file` | `results.json` | Путь к JSON-отчёту                               |

## Формат логов

Каждая строка должна иметь вид:

```
<timestamp RFC3339> [<LEVEL>] <service>: <message>
```

- уровни: `INFO`, `WARN`, `ERROR`
- `request_id=...` и `user_id=...` извлекаются из текста сообщения

Пример:

```
2024-01-15T10:30:05Z [INFO] payment-service: Payment started user_id=1001 request_id=req101
2024-01-15T10:30:09Z [ERROR] payment-service: Card declined user_id=1001 request_id=req101
```

## Примеры использования

### Анализ каталога с логами

```bash
./log-processor --input-dir ./logs --output-file report.json
```

Вывод (сокращён):

```
Scanning directory: ./logs
Found 3 log files:
  - order-service.log
  - payment-service.log
  - user-service.log

Processing files...
Starting concurrent processing with 5 workers...
Worker processed: user-service.log (4 entries)
Worker processed: payment-service.log (3 entries)
Worker processed: order-service.log (3 entries)
All workers completed.

Total entries processed: 10
Unique requests found: 8

Failed requests analysis:
Total failed requests: 4 out of 8

Report saved to report.json
```

### JSON-отчёт

```json
{
  "total_entries_processed": 10,
  "failed_requests_found": 4,
  "processing_time_seconds": 0.001,
  "failed_requests": [
    {
      "request_id": "req101",
      "failing_service": "payment-service",
      "error_message": "Card declined user_id=1001 request_id=req101",
      "timeline": [
        "10:30:05.000Z [INFO] payment-service: Payment started user_id=1001 request_id=req101",
        "10:30:09.000Z [ERROR] payment-service: Card declined user_id=1001 request_id=req101"
      ]
    }
  ]
}
```

### Прерывание по Ctrl+C

Если нажать `Ctrl+C` во время обработки, программа не теряет уже собранные данные:

```
Processing files...
Starting concurrent processing with 5 workers...
^CInterrupt signal received, shutting down...
All workers completed.
...
Partial results saved to results.json
```

## Структура проекта

```
.
├── main.go                  # точка входа: флаги, сигналы, вывод, отчёт
└── internal/
    ├── cli/                 # разбор аргументов командной строки
    ├── scanner/             # поиск *.log файлов в каталоге
    ├── parser/              # разбор строки лога в LogEntry
    ├── processor/           # чтение файлов, worker pool, корреляция и анализ
    └── reporter/            # запись JSON-отчёта
```
