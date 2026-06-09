# Архитектура — P2P Мессенджер (Fully Decentralized)

## Концепция

Никакого центрального сервера. Все устройства общаются напрямую (P2P) через WebRTC, а поиск друг друга и обмен SDP/ICE осуществляют через распределённую сеть **libp2p**.

```
┌─────────────────────────────────────────────────────────┐
│                    libp2p Network                        │
│  ┌──────────┐   DHT / mDNS / Relay   ┌──────────┐      │
│  │ Peer A   │◄══════════════════════►│ Peer B   │      │
│  │ (телефон)│  сигналинг (SDP/ICE)   │ (телефон)│      │
│  └────┬─────┘                        └────┬─────┘      │
│       │                                   │             │
│       │      ┌───────────────────────┐     │             │
│       └─────►│  WebRTC DataChannel   │◄────┘             │
│              │  (прямое P2P)         │                   │
│              │  сообщения / медиа    │                   │
│              └───────────────────────┘                   │
│                                                          │
│  ┌──────────┐                        ┌──────────┐       │
│  │ Peer C   │════════════════════════│ Peer D   │       │
│  │ (реле)   │  relay для Peer E      │ (реле)   │       │
│  └──────────┘                        └──────────┘       │
└─────────────────────────────────────────────────────────┘
```

---

## 1. Стек технологий

| Компонент | Технология | Роль |
|-----------|-----------|------|
| Flutter | Dart | UI, бизнес-логика |
| go-libp2p | Go (через gomobile) | DHT, mDNS, relay, NAT hole punching |
| flutter-webrtc | Dart | DataChannel, медиа, SRTP |
| SQLite (drift) | Dart | Локальное хранение сообщений |
| x/crypto | Go / dart | E2EE (X3DH + Double Ratchet) |

### Как связываются Go и Dart

```
┌────────────────────┐
│   Flutter (Dart)   │
│  ┌──────────────┐  │
│  │ flutter-webrtc│  │
│  │ DataChannel   │  │
│  └──────┬───────┘  │
│         │ Method   │
│         │ Channel  │
│  ┌──────┴───────┐  │
│  │ go-libp2p    │  │
│  │ (gomobile)   │  │
│  │   .so/.aar   │  │
│  └──────────────┘  │
└────────────────────┘
```

Go-код компилируется в нативную библиотеку (`.aar` для Android, `.xcframework` для iOS) и взаимодействует с Dart через Platform Channels / Method Channel.

---

## 2. libp2p — децентрализованный сигналинг

libp2p заменяет собой весь signaling server. Каждое устройство запускает libp2p node.

### 2.1 Компоненты libp2p

```
┌──────────────────────────────────────────────┐
│              libp2p Host                      │
│                                                │
│  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │ Identity │  │  DHT     │  │   mDNS     │  │
│  │ (PeerID) │  │(Kademlia)│  │(LAN disc.) │  │
│  │ Ed25519  │  │          │  │            │  │
│  └──────────┘  └──────────┘  └────────────┘  │
│                                                │
│  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Relay   │  │  PubSub  │  │  Identify  │  │
│  │ (circuit)│  │(floodsub)│  │  Protocol  │  │
│  └──────────┘  └──────────┘  └────────────┘  │
│                                                │
│  ┌──────────────────────────────────────────┐  │
│  │         Connection Manager               │  │
│  │  автоматическое управление соединениями │  │
│  └──────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

### 2.2 Peer Identity

```dart
// Генерируется один раз при первом запуске
// Хранится в локальном хранилище

PeerID = base58(sha256(public_key))
private_key = Ed25519PrivateKey.generate()

// PeerID — единственный идентификатор пользователя
// Пример: 12D3KooW... (mulithash)
```

### 2.3 DHT (Kademlia) — поиск по интернету

```
// Каждый пир хранит часть распределённой таблицы
// Ключ: PeerID получателя
// Значение: multiaddr (адрес пира)

// Поиск пира:
1. Вычисляем XOR-дистанцию до PeerID цели
2. Запрашиваем у известных пиров ближайших к цели
3. Повторяем, пока не найдём целевой пир
4. Получаем его multiaddr

// Bucket (KBucket):
каждый bucket хранит до K = 20 пиров
расстояние: XOR(PeerID_A, PeerID_B)
```

### 2.4 mDNS — обнаружение в LAN

```
// При запуске:
- libp2p регистрирует mDNS сервис _p2p._udp
- Каждые 30 секунд отправляет multicast запрос

// При обнаружении пира:
- Получаем его PeerID + multiaddr
- Добавляем в адресную книгу
- Пытаемся установить прямое соединение

// Задержка: 100-500ms (в локальной сети)
// Не требует интернета
```

### 2.5 Relay — когда нет прямого соединения

```
// Если NAT не позволяет прямое соединение:

Peer A ────Соединение через реле──── Peer B
    │                                      │
    └── libp2p circuit relay (via Peer C) ──┘

// Relay discovery:
Peer A находит Peer C через DHT или mDNS
Peer C — обычное устройство с открытым портом
Peer A запрашивает у Peer C релейное соединение до Peer B

// Circuit relay v2:
- Ограничение по трафику (лимит на реле)
- Автоматический выбор реле
- Reservation — пир резервирует слот на реле
```

### 2.6 Protocol ID для сигналинга

```
/webrtc-signaling/1.0.0     — SDP + ICE exchange
/webrtc-msg/1.0.0           — сообщения (текст)
/webrtc-media/1.0.0         — медиа chunks
/webrtc-e2ee/1.0.0          — E2EE key exchange
```

---

## 3. Поток соединения (Fully P2P)

```
Peer A (Flutter)              libp2p Network              Peer B (Flutter)
     │                             │                           │
     │  ── libp2p.Start() ────────►│◄─── libp2p.Start() ──── │
     │                             │                           │
     │  ── DHT.FindPeer(B) ───────►│                           │
     │◄───── multiaddr(B) ────────│                           │
     │                             │                           │
     │  ── Direct Connect(B) ─────│──────────────────────────►│
     │◄══════ WebRTC Offer (SDP) ═══════════════════════════►│
     │◄══════ WebRTC Answer (SDP) ═══════════════════════════►│
     │◄══════ ICE Candidates ═══════════════════════════════►│
     │                             │                           │
     │◄═══════════ ICE + DTLS + SRTP/SCTP ══════════════════►│
     │                             │                           │
     │  ── DataChannel.send(msg) ────────────────────────────►│
     │◄── DataChannel.send(msg) ──────────────────────────────│
```

### Если прямое соединение невозможно:

```
Peer A           libp2p Circuit Relay (Peer C)           Peer B
  │                        │                               │
  │── Connect(C) ────────►│◄──── Connect(C) ────────────── │
  │── Relay.Reserve ─────►│◄──── Relay.Reserve ─────────── │
  │                        │                               │
  │── Relay.Connect(B) ──►│── Relay.Connect(A) ──────────►│
  │◄═══════ SDP/ICE через Relay ═════════════════════════►│
  │                        │                               │
  │◄═══════ WebRTC через реле (TURN-like) ═══════════════►│
```

---

## 4. NAT Traversal стратегия (без сервера)

```
Попытка соединения:
    │
    ├── LAN (mDNS)?
    │   └── Прямое TCP/WebRTC — 100% успех
    │
    ├── DHT дал multiaddr?
    │   ├── Прямое соединение (hole-punch) — 70-80%
    │   └── Неудача → Circuit Relay через другой пир — 99%
    │
    └── Peer не найден в DHT?
        └── Онлайн, но недоступен → PubSub notify "connect me"
```

### 4.1 Hole Punching (через libp2p)

```
// libp2p hole-punching с использованием relay:

1. A и B подключаются к общему реле (C)
2. A и B обмениваются multiaddr через реле
3. A и B одновременно отправляют пакеты друг другу
4. NAT открывает дырки с обеих сторон
5. Прямое соединение установлено
6. Реле (C) больше не участвует
```

### 4.2 Bootstrap пиры

```
// Для входа в DHT сеть нужны начальные точки входа:

// Варианты bootstrap:
1. Публичные bootstrap ноды (как в IPFS)
   /dns/bootstrap.libp2p.io/tcp/9091/p2p/...
   /ip4/147.75.83.83/tcp/4001/p2p/...

2. Ранее известные пиры (из адресной книги)

3. Знакомые по QR-коду / NFC

// После подключения к DHT — пиры находят друг друга
// без bootstrap сервера
```

---

## 5. Сообщения (поверх WebRTC DataChannel)

### 5.1 Протокол сообщений

```
// Все сообщения идут через DataChannel (SCTP)
// после успешного WebRTC соединения

Message {
  type:    "text" | "image" | "video" | "file" | "e2ee_key" | "receipt"
  id:      UUID
  from:    PeerID (base58)
  to:      PeerID (base58)
  payload: bytes (зашифрованные)
  ack:     bool  // требуется ли подтверждение
  ts:      int64 // unix timestamp
}
```

### 5.2 Типы сообщений

| Тип | DataChannel | Шифрование | Описание |
|-----|------------|------------|----------|
| `text` | reliable | E2EE | Текстовое сообщение |
| `image` | unreliable | E2EE | Сжатое WebP изображение |
| `video` | unreliable | E2EE | Видео chunk |
| `file` | reliable | E2EE | Файл (бинарный) |
| `e2ee_key` | reliable | нет* | Обмен ключами X3DH |
| `receipt` | reliable | нет | Квитанция (delivered/read) |

\* `e2ee_key` не шифруется содержимым, но идёт по зашифрованному DataChannel

### 5.3 Квитанции

```
A ── msg(id=1, ack=true) ──────────────────► B
A ◄── receipt(id=1, status=delivered, ts) ── B

// Позже:
A ◄── receipt(id=1, status=read, ts) ──────── B
```

---

## 6. Сжатие медиа (на клиенте)

Без изменений относительно предыдущей версии:

- **Изображения**: `dart:ui` decodeImage → resize → WebP 0.8
- **Видео**: MediaRecorder (flutter-webrtc) с адаптивным битрейтом
- **Выигрыш**: 10-25x для изображений, 5-20x для видео

---

## 7. База данных SQLite (drift)

```
┌───────────────────────────────────────────────────────────┐
│                   SQLite (drift)                           │
│                                                            │
│  peers:         peer info, username, avatar, pub_key      │
│  conversations: last message, unread count, peerID        │
│  messages:      text, media, status, timestamps           │
│  media_cache:   local paths, hash, size                   │
│  key_store:     E2EE keys (зашифрованы мастер-ключом)     │
└───────────────────────────────────────────────────────────┘
```

---

## 8. E2EE (Signal Protocol)

```
// X3DH — установка initial ключа
// Double Ratchet — смена ключей на каждое сообщение

Сообщение → XChaCha20-Poly1305 → DataChannel

// Ключи хранятся в SQLite (key_store)
// Мастер-ключ выводится из PIN-кода/биометрии
```

---

## 9. Структура директорий

```
messenger/
│
├── client/                          # Flutter
│   ├── lib/
│   │   ├── main.dart
│   │   ├── app.dart
│   │   │
│   │   ├── core/
│   │   │   ├── p2p/                 # Мост к go-libp2p
│   │   │   │   ├── libp2p_bridge.dart    # Method Channel
│   │   │   │   ├── libp2p_bridge.g.dart  # автогенерация
│   │   │   │   └── models.dart
│   │   │   ├── constants.dart
│   │   │   └── utils.dart
│   │   │
│   │   ├── data/
│   │   │   ├── database/
│   │   │   │   ├── database.dart
│   │   │   │   ├── tables.dart
│   │   │   │   └── daos/
│   │   │   ├── repositories/
│   │   │   └── models/
│   │   │
│   │   ├── services/
│   │   │   ├── libp2p_service.dart  # обёртка над bridge
│   │   │   ├── webrtc_service.dart
│   │   │   ├── media_service.dart
│   │   │   ├── message_service.dart
│   │   │   └── crypto_service.dart
│   │   │
│   │   ├── blocs/
│   │   │   ├── auth/
│   │   │   ├── chat/
│   │   │   └── contacts/
│   │   │
│   │   └── ui/
│   │       ├── screens/
│   │       └── widgets/
│   │
│   └── test/
│
├── p2p-node/                        # Go (libp2p) — нативный модуль
│   ├── cmd/
│   │   └── mobile/
│   │       └── main.go              # gomobile entrypoint
│   │
│   ├── p2p/
│   │   ├── node.go                  # libp2p host
│   │   ├── dht.go                   # Kademlia
│   │   ├── discovery.go             # mDNS
│   │   ├── relay.go                 # circuit relay v2
│   │   ├── signaling.go             # WebRTC signal protocol
│   │   └── holepunch.go             # NAT hole punching
│   │
│   ├── bridge/
│   │   └── bridge.go                # Method Channel API
│   │
│   ├── crypto/
│   │   ├── x3dh.go
│   │   └── ratchet.go
│   │
│   ├── go.mod
│   └── go.sum
│
├── docs/
│   ├── architecture.md
│   ├── webrtc.md
│   └── ...
│
└── README.md
```

---

## 10. План реализации (фазы)

| Фаза | Задачи | Результат |
|------|--------|-----------|
| **1. libp2p node** | Go: node, DHT, mDNS, relay, holepunch. gomobile сборка | Два устройства находят друг друга в LAN и через интернет |
| **2. WebRTC bridge** | Flutter: libp2p_service → webrtc_service | SDP/ICE через libp2p → установка DataChannel |
| **3. Сообщения** | Text messages через DataChannel, SQLite | Текстовый чат P2P |
| **4. E2EE** | X3DH + Double Ratchet на Go | Сквозное шифрование |
| **5. Медиа** | Сжатие + отправка изображений/видео | Файлообмен P2P |
| **6. UI** | Полный интерфейс мессенджера | Рабочее приложение |

---

## 11. Bootstrap — как войти в сеть без сервера

```
Первый запуск:
  1. Генерируется PeerID (Ed25519)
  2. Адресная книга пуста
  3. mDNS слушает LAN (найдутся соседи)
  4. DHT неактивна (нет контактов)

Способы получения первых контактов:
  ┌──────────────────────────────────────┐
  │ QR-код / NFC                         │
  │ Два телефона рядом → сканирование →  │
  │ обмен PeerID + multiaddr             │
  │ → DHT контакт                         │
  └──────────────────────────────────────┘
  ┌──────────────────────────────────────┐
  │ Встроенные bootstrap пиры (опционально)│
  │ Несколько публичных нод в DHT         │
  │ (можно сделать на старых устройствах) │
  └──────────────────────────────────────┘
  ┌──────────────────────────────────────┐
  │ Приглашение по ссылке                 │
  │ "https://invite/12D3KooW..."         │
  │ Получатель добавляет пира вручную    │
  └──────────────────────────────────────┘
```

---

## 12. Плюсы и минусы подхода

| Плюсы | Минусы |
|-------|--------|
| Нет сервера — нет затрат на хостинг | Bootstrap без интернета — только LAN |
| Нет единой точки отказа | Relay потребляет трафик реле-пиров |
| Приватность (никто не видит метаданные) | DHT latency (поиск пира 1-3 сек) |
| Работает в LAN без интернета | Сложность разработки (Go + Dart) |
| E2EE из коробки | gomobile ограничения (не весь Go) |
