# VK-inretnship-GO
Демонстрационный сервис gRPC Pub/Sub на Go:

- Встроенная шина `SubPub` (in-memory, FIFO, с буферизацией, изоляцией «медленных» подписчиков).
- gRPC API с методами:
    - `Publish(subject, data)` — отправка сообщения в тему.
    - `Subscribe(subject)` — подписка на поток сообщений.
    - `Health()` — простая проверка «живости».
- Конфигурация через `configs/config.yaml` (порт, размер буфера, таймаут graceful-shutdown).
- Логирование с помощью `go.uber.org/zap`.
- Unit- и интеграционные тесты для шины и сервера.
- Контейнеризация с Docker и Makefile для автоматизации.

## Структура проекта
```
VK-inretnship-GO/
├── api/
│   └── serv/
│       └── v1/
│           ├── pubsub.pb.go
│           └── pubsub_grpc.pb.go
├── cmd/
│   └── server/
│       └── main.go
├── configs/
│   └── config.yaml
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── server/
│   │   ├── pubsub.go
│   │   └── pubsub_test.go
│   └── subpub/
│       ├── subpub.go
│       └── subpub_test.go
├── proto/
│   └── pubsub.proto
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

## Сборка и запуск
### 1. Клонирование
```bash
git clone https://github.com/andriichukmo/VK-inretnship-GO.git
cd VK-inretnship-GO
```
### 2. Установка внешних зависимостей
```bash
chmod +x scripts/setup.sh
./scripts/setup.sh
```
### 3. Установка зависимостей
```bash
go mod tidy
```
### 4. Генерация gRPC-стабов из `.proto`
```bash
make proto
```
### 5. Запуск сервера
```bash
make run
```
По умолчанию сервер случает `:50051`.
### 6. Проверка
#### проверка, что сервер работает
```bash
grpcurl -plaintext localhost:50051 serv.v1.PubSubService/Health
```
(Если изменялся порт, который слушает сервер, то надо заменить 50051 на то, на что был заменен порт)
#### Проверка того, что сообщения отправляются и приходят

Необходимо хотя бы 2 терминала. На первом подписываемся:
```bash
grpcurl -plaintext \
  -d '{"subject":"demo"}' \
  localhost:50051 serv.v1.PubSubService/Subscribe
```
На втором отправляем сообщение:
```bash
grpcurl -plaintext \
  -d '{"subject":"demo","data":"aGVsbG8="}' \
  localhost:50051 serv.v1.PubSubService/Publish
```
где `aGVsbG8=` - это "hello" в base64.
Тогда на первом терминале должно появиться:
```
{
    "data": "aGVsbG8="
}
```

## Makefile
| Цель | Описание |
|-----|-------|
| make proto | Генерация Go-кода из proto-файла |
| make test | `go test -race ./...` |
| make build | Сборка бинаря `vk-server` |
| make run | `make proto build`+ `go run ./cmd/server`
| make docker | `make proto build` + `docker build vk-server:latest` |
| make clean | Удалить бинарь `vk-server` |

## Docker

### 1. Сборка
```bash
make docker
```
Создастся образ `vk-server:latest`
### 2. Запуск контейнера
```bash
docker run --rm -d -p 50051:50051 --name vk-server vk-server:latest
```
И вновь, если порт менялся, то надо заменить и 50051 на то, на что заменили.
### 3. Остановка
```bash
docker stop vk-server
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

### на чем тестировался:

| Тест | Что проверяет |
|------|---------------|
| TestPublishOrderFIFO | 100 сообщений доходят до подписчика в точном порядке (First In First Out) |
| TestSlowSubscriberIsolation | Медленный consumer (реализованный при помощи Sleep 5 ms) не мешает быстрому — быстрый принимает все 50 сообщений. |
| TestUnsubscribeStopDelivery | После ``Unsubscribe`` новые сообщения не доставляются. |
| TestErrorWithClosed | И ``Subscribe``, и ``Publish`` возвращают ошибку после ``Close`` |
| TestConcurrentFIFO | 10 параллельных подписчиков: каждый получает 200 сообщений без нарушения порядка. |
| **Graceful Shutdown** | `Close(ctx)` помечает bus закрытым, отменяет контексты подписчиков. Пока `ctx` жив, ждёт завершения воркеров; отмена `ctx` — выходит сразу, горутины дочитывают буферы. |


## пакет config
### Публичное API
```go
// Config хранит параметры приложения
type Config struct {
    GRPC struct {
        Addr string          // адрес для Listen()
    }
    Queue struct {
        Buffer int           // размер буфера шины
    }
    ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"` // таймаут graceful-shutdown
}

func Load() (*Config, error) // читает configs/config.yaml и возвращает конфиг 
```
### Что он делает
| Возможность | Детали реализации |
|-------------|-|
| Загрузка YAML | Использует Viper, ищет файл `configs/config.yaml` |
| Дефолтные значения | Есть дефолтные значения для всех параметров, так что даже если с файлом что-то произойдет, проект всё равно соберется (хоть и с дефолтными значениями) |

## пакет server
### Публичное API
```go
// Конструктор сервиса
func NewPubSubService(bus subpub.SubPub, log *zap.Logger) servv1.PubSubServiceServer

// Сгенерированный интерфейс gRPC
type PubSubServiceServer interface {
    Publish(context.Context, *servv1.PublishRequest) (*emptypb.Empty, error)
    Subscribe(*servv1.SubscribeRequest, servv1.PubSubService_SubscribeServer) error
    Health(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
}
```
### Что он делает
| Возможность | Детили реализации |
|-------------|-------------------|
| Publish → SubPub | Вызов `bus.Publish(subject, data)` и возврат Empty или ошибки |
| Subscribe → поток | `bus.Subscribe(...)` создаёт callback, в котором `stream.Send(...)` отправляет `[]byte` без копий |
| Health | Простой `Empty → Empty` для проверки того, что сервер работает |
| gRPC Reflection | `reflection.Register(grpcServer)` для grpcurl и IDE |
| Graceful shutdown | Сервер вызывает `GracefulStop()`, затем `bus.Close(ctx)`, дожидаясь завершения горутин. |
### на чем тестировался
| Тест | Что проверяет |
|------|---------------|
| TestPublishSubscribe | Проверяет, что можно отправить данные и подписчик получает нужные данные. |
| TestPublishAfterStop | После `GracefulStop()` или `cleanup()` `Publish` выдаёт `Unavailable` или Canceled. |

## cmd/server (CLI)
### публичное поведение
- Читает конфиг через `config.Load()`.
- Создаёт `subpub.NewSubPubWithParams(buffer, logger)`.
- Запускает gRPC-сервер на `cfg.GRPC.Addr`.
- Регистрирует сервис и reflection.
- Ловит SIGINT/SIGTERM, делает `GracefulStop()` и `bus.Close(ctx)`.

## Что логируется, а что нет

### пакет subpub (`internal/subpub`)
| Событие | Уровень  | Сообщение и поля |
|-|-|-|
| **Добавление подписчика** | DEBUG | `logger.Debug("subscriber added", zap.String("subject", key), zap.Int("buffer", b.buffer))` |
| **Отписка подписчика** | DEBUG | `logger.Debug("unsubscribe", zap.String("subject", s.subjkey))`|
| **Удаление после Unsubscribe** | DEBUG | `logger.Debug("subscriber removed", zap.String("subject", key))` |
| **Publish в закрытую шину** | WARN | `logger.Warn("publish to closed bus", zap.String("subject", key))` |
| **Subscribe к закрытому bus** | WARN | `logger.Warn("subscribe to closed bus", zap.String("subject", key))` |
| **Медленный подписчик (буфер переполнен)** | WARN | `logger.Warn("Slow subscriber", zap.String("subject", key))` |
| **Отмена медленного подписчика** | WARN | `logger.Warn("Canceled slow subscriber", zap.String("subject", key))`|
| **Успешное завершение Close** | INFO | `logger.Info("bus closed", zap.Int("subscribers", len(allSub)))`|
| **Таймаут Close** | WARN | `logger.Warn("bus close timeout", zap.Error(ctx.Err()))` |

### пакет server (`internal/server`)
| Событие | Уровень | Сообщение и поля |
|-|-|-|
| **Publish RPC** | — | не логируется напрямую, ошибки возвращаются в gRPC-клиент |
| **Subscribe RPC** | — | не логируется (streaming внутри gRPC) |
| **Старt gRPC-сервера** | INFO | `logger.Info("starting gRPC server", zap.String("address", cfg.GRPC.Addr))` |
| **Не удалось Listen** | FATAL | `logger.Fatal("failed to listen", zap.Error(err))` |
| **Ошибка Serve** | FATAL | `logger.Fatal("failed to serve gRPC", zap.Error(err))` |
| **Начало graceful-shutdown** | INFO | `logger.Info("shutting down gRPC server")` |
| **RPC после остановки сервера** | — | возвращается gRPC-код `Unavailable`/`Canceled`, не логируется |

### cmd/server (точка входа)
| Событие | Уровень | Сообщение и поля |
|-|-|-|
| **Загрузка конфига** | — | используют `log.Fatal("failed to load config", zap.Error(err))` при ошибке |
| **Создание шины, логгер** | — | не логируется |
| **Запуск сервера в горутине** | INFO | `logger.Info("starting gRPC server", zap.String("address", grpcAddr))` |
| **GracefulStop** | INFO | при успешном завершении `bus.Close` шина логирует INFO "bus closed" |
| **Ошибка закрытия шины** | WARN | при таймауте `Close(ctx)` шина логирует WARN "bus close timeout" |

