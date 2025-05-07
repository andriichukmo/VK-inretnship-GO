# VK-inretnship-GO
Pub/Sub Service

## Сборка и запуск
```bash
make build        # бинарь в ./bin
make run          # запускает gRPC‑сервер с graceful shutdown
make test         # go test ./...
```

## пакет subpub

### публичное API
```go
// Конструкторы
func NewSubPub() SubPub                                         // buffer = 1024, logger = zap.NewNop()
func NewSubPubWithParams(buffer int, logger *zap.Logger) SubPub // полный контроль

// Типы
type MessageHandler func(msg interface{})

// Главный интерфейс
type SubPub interface {
    Subscribe(subject string, cb MessageHandler) (Subscription, error)
    Publish(subject string, msg interface{}) error
    Close(ctx context.Context) error
}

// Объект возврата Subscribe
type Subscription interface {
    Unsubscribe()
}
```

### что он делает:
| Возможность | Детали реализации |
|-------------|------------------|
| **Асинхронная доставка (fan‑out)** | Для каждого `subject` хранится набор подписчиков; сообщение копируется в личный буфер каждого. |
| **Гарантия FIFO** | Порядок сообщений сохраняется внутри каждого подписчика. |
| **Изоляция «медленных»** | Медленный хендлер блокирует только свой канал — остальные продолжают работу. |
| **Без утечек** | Все воркеры учитываются в ``sync.WaitGroup``; ``go test -race`` проходит без утечек. |
| **Логирование** | Логируются подписка/отписка, попытки работы с закрытым bus, закрытие. |

### на чем тестировался:

| Тест | Что проверяет |
|------|---------------|
| TestPublishOrderFIFO | 100 сообщений доходят до подписчика в точном порядке (First In First Out) |
| TestSlowSubscriberIsolation | Медленный consumer (реализованный при помощи Sleep 5 ms) не мешает быстрому — быстрый принимает все 50 сообщений. |
| TestUnsubscribeStopDelivery | После ``Unsubscribe`` новые сообщения не доставляются. |
| TestErrorWithClosed | И ``Subscribe``, и ``Publish`` возвращают ошибку после ``Close`` |
| TestConcurrentFIFO | 10 параллельных подписчиков: каждый получает 200 сообщений без нарушения порядка. |

### как запустить тесты

```bash
go test ./internal/subpub/ -race
```



