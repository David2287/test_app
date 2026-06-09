# WebRTC Protocol — Complete Specification

## 1. Общая архитектура и сигналинг

```
Peer A                    Signaling Server                    Peer B
   |                            |                               |
   |------- SDP Offer -------->|                               |
   |                            |------- SDP Offer ----------->|
   |                            |<------ SDP Answer -----------|
   |<------ SDP Answer --------|                               |
   |------- ICE Candidates --->|                               |
   |                            |------- ICE Candidates ------>|
   |<------ ICE Candidates ----|                               |
   |                                                           |
   |<================= DTLS Handshake ========================>|
   |<=================== SRTP/SCTP ===========================>|
```

- **SDP (Session Description Protocol)**: описывает медиа-сессию (кодеки, ICE-кандидаты, fingerprint сертификата)
- **Signaling**: не специфицируется WebRTC — может быть WebSocket, SIP, XMPP
- **JSEP (JavaScript Session Establishment Protocol)**: отделяет логику сигналинга от медиа

---

## 2. ICE (Interactive Connectivity Establishment)

### 2.1 Формула приоритета кандидатов

```
priority = (2^24) * type_pref + (2^8) * local_pref + (256 - component_id)
```

**Типы кандидатов и их type_pref:**

| Тип | type_pref | Описание |
|-----|-----------|----------|
| host | 126 | Локальный IP |
| prflx | 110 | Peer-reflexive (от STUN) |
| srflx | 100 | Server-reflexive |
| relay | 0 | Релейный (TURN) |

- `local_pref`: предпочтение конкретного интерфейса (обычно 65535)
- `component_id`: 1 для RTP, 2 для RTCP

### 2.2 Connectivity Checks

```
num_pairs = |A_candidates| * |B_candidates|
total_time = num_pairs * (RTT + 200ms)
```

- Каждая пара кандидатов проверяется STUN request/response
- **Triggered check**: при получении STUN-запроса для неизвестной пары
- **Nominated**: флаг, указывающий что пара выбрана для медиа
- **ICE restart**: новый ICE-UFrag/ICE-Pwd, сброс всех пар

### 2.3 ICE State Machine

```
ICE State:
  New → Gathering → Complete
    ↓                    ↓
  Checking ──────────→ Connected → Completed
    ↓                    ↓
  Closed               Disconnected → Failed
```

---

## 3. STUN (Session Traversal Utilities for NAT)

### 3.1 Формат сообщения

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|0 0|     STUN Message Type     |         Message Length        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         Magic Cookie                          |
|                          (0x2112A442)                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                     Transaction ID (96 bits)                  |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.2 Message Integrity

```
Key = MD5(credentials)
HMAC = HMAC-SHA1(Key, message_type || length || cookie || tx_id || attributes)

// Fingerprint
Fingerprint = CRC32(stun_message_without_fingerprint) ^ 0x5354554E
```

- Transaction ID: 96-битный случайный идентификатор
- Magic Cookie: `0x2112A442` (32 бита)
- **STUN timer backoff**: `RTO = max(500ms, RTO_prev * 2)`, до 7 попыток

### 3.3 Классификация NAT

| Тип NAT | Описание | ICE проходимость |
|---------|----------|-----------------|
| Full Cone | Любой внешний → внутренний | ~99% |
| Address Restricted | Только ранее отправлявшие | ~90% |
| Port Restricted | По IP+Port | ~70% |
| Symmetric | Разный внешний порт на каждый destination | ~0% (без TURN) |

---

## 4. DTLS 1.2 — Handshake и Key Derivation

### 4.1 Handshake Flow

```
Client                                     Server
   |                                          |
   |------ ClientHello ---------------------->|
   |                                        (выбирает cipher)
   |<----- ServerHello ----------------------|
   |<----- Certificate ----------------------|
   |<----- ServerKeyExchange (ECDHE) --------|
   |<----- CertificateRequest (optional) -----|
   |<----- ServerHelloDone ------------------|
   |                                        |
   |------ Certificate --------------------->|
   |------ ClientKeyExchange (ECDHE) ------->|
   |------ CertificateVerify --------------->|
   |------ ChangeCipherSpec ----------------->|
   |------ Finished ------------------------->|
   |                                        |
   |<----- ChangeCipherSpec -----------------|
   |<----- Finished -------------------------|
   |                                        |
   |<================= SRTP DATA ============>|
```

### 4.2 ECDHE

```
// Domain parameters (X25519):
p = 2^255 - 19
x = X25519(private_key, basepoint)

// Shared secret:
shared_secret = d_A * Q_B = d_B * Q_A
```

- P-256 (secp256r1) и X25519 — обязательные кривые
- ECDHE обеспечивает PFS (Perfect Forward Secrecy)

### 4.3 Key Derivation (TLS PRF)

```
// Master Secret:
master_secret = PRF(pre_master_secret, "master secret",
                    client_random + server_random)[0..47]

// Key Block:
key_block = PRF(master_secret, "key expansion",
                server_random + client_random)

// Распределение:
client_write_MAC_key = key_block[0..19]    // 20 bytes (SHA1)
server_write_MAC_key = key_block[20..39]
client_write_key     = key_block[40..55]   // 16 bytes (AES-128)
server_write_key     = key_block[56..71]
client_write_IV      = key_block[72..75]   // 4 bytes
server_write_IV      = key_block[76..79]
```

### 4.4 SRTP Key Extraction (DTLS-SRTP)

```
// Используется DTLS handshake для согласования SRTP-ключей

srtp_master_key = PRF(master_secret, "EXTRACTOR-dtls_srtp",
                      client_random + server_random)

// Для AES-128:
SRTP_AES_128:
  encryption_key = srtp_master_key[0..15]   // 128 bit
  salt_key       = srtp_master_key[16..29]  // 112 bit

// Для AES-256:
SRTP_AES_256:
  encryption_key = srtp_master_key[0..31]   // 256 bit
  salt_key       = srtp_master_key[32..45]  // 112 bit

// HKDF Expansion:
srtp_session_key = HKDF-Expand(srtp_master_key, label, salt, L)
```

---

## 5. SRTP/SRTCP — Шифрование медиа

### 5.1 AES-CTR Mode

```
ciphertext = plaintext ⊕ AES(key, IV, index)
```

**IV Derivation:**
```
IV[0..3]  = salt[0..3]  ⊕ (ROC >> 0)
IV[4..7]  = salt[4..6]  ⊕ (SSRC >> 8) || (ROC >> 24)
IV[8..11] = salt[8..10] ⊕ (SSRC >> 40) || (index >> 8)
IV[12..15]= salt[12..14]⊕ (index >> 40)

// Simplified:
IV = (salt * 2^16) | ROC | (SSRC * 2^64) | index
```

- **ROC (Rollover Counter)**: 32-bit counter, инкрементится при переполнении sequence number
- **Sequence number**: 16-bit в RTP заголовке
- `index = ROC * 2^16 + seq_num`

### 5.2 Authentication Tag

```
auth_tag = HMAC-SHA1(srtp_session_salt, packet_data)[0..9]  // 80 bit
```

- SRTP auth tag: 4 байта (обязательно)
- SRTCP auth tag: 80 бит (обязательно)
- MKI (Master Key Identifier): опционально

### 5.3 Packet Layout

```
SRTP Packet:
+----------------+----------------+----------------+-----------+
| RTP Header (12) | Encrypted Payload  | Auth Tag (4) | MKI (opt) |
+----------------+----------------+----------------+-----------+

SRTCP Packet:
+----------------+----------------+----------------+-----------+
| RTCP Header     | Encrypted Payload | Auth Tag (10) | MKI (opt) |
+----------------+----------------+----------------+-----------+
```

---

## 6. SCTP — Транспорт DataChannel

### 6.1 Ассоциация (4-way handshake)

```
Peer A                               Peer B
   |                                    |
   |----------- INIT ------------------>|   (Initiate Tag, a_rwnd, OS/IS)
   |<---------- INIT-ACK ---------------|   (Tag, a_rwnd, OS/IS, cookie)
   |----------- COOKIE-ECHO ----------->|   (State Cookie)
   |<---------- COOKIE-ACK -------------|   (Подтверждение)
   |                                    |
   |<=========== DATA/SACK =============>|
```

### 6.2 Selective ACK (SACK)

```
SACK chunk:
  Cumulative TSN Ack
  Advertised Receiver Window Credit (a_rwnd)
  Number of Gap Ack Blocks
  Number of Duplicate TSNs
  Gap Ack Blocks: [Start, End] pairs
  Duplicate TSNs
```

### 6.3 Congestion Control

```
// Slow Start:
for each ACK:
    cwnd += min(n, MTU)   // n — количество новых байт

// Congestion Avoidance:
for each ACK:
    cwnd += MTU * MTU / cwnd

// или (RFC 4960 эквивалент):
cwnd += min(MTU, bytes_acked / cwnd * MTU)

// Fast Retransmit:
ssthresh = max(cwnd / 2, 4 * MTU)
cwnd = ssthresh

// RTO Calculation:
SRTT = (1 - α) * SRTT + α * RTT_sample         // α = 1/8
RTTVAR = (1 - β) * RTTVAR + β * |SRTT - RTT_sample|  // β = 1/4
RTO = SRTT + 4 * RTTVAR
```

### 6.4 Data Channel Parameters

| Параметр | Значение | Описание |
|----------|----------|----------|
| Max message size | 256 KB | По умолчанию |
| Ordered delivery | true/false | Настраивается |
| Max retransmits | 0-65535 | -1 для бесконечности |
| Max lifetime | 0-65535 ms | -1 для бесконечности |

---

## 7. GCC (Google Congestion Control)

### 7.1 Общая архитектура

```
Remote Bitrate Estimator (Kalman Filter):
  - Delay-based: оценивает задержки между пакетами
  - Loss-based: реагирует на потери

Итоговый битрейт: A = min(As, Ar)

где:
  As — битрейт от loss-based контроллера
  Ar — битрейт от delay-based контроллера
```

### 7.2 Kalman Filter Delay Estimation

```
// Состояние:
θ = [1/γ, m]   — вектор состояния
  γ — скорость канала (битрейт)
  m — шумовая компонента задержки

// Наблюдение:
d_f(i) = h(i)^T · θ(i) + ε

где:
  d_f(i) — inter-arrival jitter
  h(i) = [δ_t(i), δ_s(i)]^T
  δ_t(i) — временной интервал между пакетами
  δ_s(i) — разница в размерах пакетов
  ε — гауссовский шум

// Prediction:
θ(i|i-1) = θ(i-1)
P(i|i-1) = P(i-1) + Q

где:
  P — ковариационная матрица ошибки
  Q — ковариация шума процесса

// Gain:
K = P(i|i-1) · h(i) · (h(i)^T · P(i|i-1) · h(i) + R)^(-1)

где:
  R — ковариация шума наблюдения

// Update:
θ(i) = θ(i) + K · (d_f(i) - h(i)^T · θ(i))
P(i) = (I - K · h(i)^T) · P(i|i-1)
```

### 7.3 Over-use Detection

```
// Тренд задержки:
m(i) ≥ m(i-1) + k · σ(i) → over-use

где:
  m(i) — текущая оценка шума
  σ(i) — стандартное отклонение
  k — коэффициент чувствительности (обычно 2-3)

// Если over-use:
Ar = Ar · 0.85

// Если норма:
Ar += α · (target_rate - Ar)  // α ≈ 0.1
```

### 7.4 Loss-based Rate Control

```
// Если потери > 2%:
As = As · (1 - 0.5 * sqrt(p))

где p — доля потерянных пакетов

// Если потери < 2%:
As = As · 1.05  // moderate increase

// Packet loss ratio:
p = lost_packets / total_packets
```

### 7.5 Rate Allocation

```
// Итоговый битрейт:
A = min(As, Ar)

// Адаптация:
if (over-use && A < prev_A):
    A = A * beta     // beta = 0.85

if (under-use):
    A = A * 1.05    // осторожный рост

// Ограничения:
A ∈ [min_bitrate, max_bitrate]
min_bitrate ≈ 30000 bps (video)
max_bitrate ≈ 2.5 Mbps (default)
```

---

## 8. Opus — Математика аудиокодека

### 8.1 SILK (речевой режим)

```
// LPC (Linear Predictive Coding):
s[n] = Σ a_k * s[n - k] + e[n]    k = 1..p

где:
  s[n] — сэмпл сигнала
  a_k — LPC коэффициенты
  e[n] — сигнал ошибки (возбуждение)
  p — порядок LPC (обычно 12-16)

// LSF (Line Spectral Frequencies) — устойчивое представление LPC:
A(z) = 1 + a₁·z⁻¹ + ... + aₚ·z⁻ᵖ
P(z) = A(z) + z⁻⁽ᵖ⁺¹⁾ · A(z⁻¹)
Q(z) = A(z) - z⁻⁽ᵖ⁺¹⁾ · A(z⁻¹)

// Корни P(z) и Q(z) = LSF
```

### 8.2 CELT (музыкальный режим)

```
// MDCT (Modified Discrete Cosine Transform):
X[k] = Σ w[n] · x[n] · cos(π/N · (n + 0.5 + N/2) · (k + 0.5))
     n=0..2N-1

где:
  x[n] — временной сэмпл (с перекрытием 50%)
  w[n] — окно (принцип Princen-Bradley)
  N — размер кадра

// Pyramidal Vector Quantization (PVQ):
R = ||x||₁ / norm

Кодирование вектора на сфере:
  n-мерный вектор → нормализация → разбиение на пирамиду
```

### 8.3 Range Coding

```
// Энтропийное кодирование с адаптивной моделью:

// Probability update:
P[symbol] = P[symbol] * (1 - α) + count * α  // α ≈ 0.01

// Range:
low = low + range * cum_prob[symbol]
range = range * prob[symbol]
```

### 8.4 FEC (Forward Error Correction)

```
// In-band FEC:
frame_n: содержит основной пакет + сжатый frame_(n-1)
// PLC (Packet Loss Concealment):
s[n] = Σ a_k · s[n - k]   // экстраполяция LPC
```

### 8.5 Параметры Opus

| Режим | Битрейт | Задержка | Применение |
|-------|---------|----------|------------|
| SILK (NB) | 8-12 kbps | 20 ms | Телефония |
| SILK (WB) | 12-20 kbps | 20 ms | Речь VoIP |
| Hybrid | 20-36 kbps | 20 ms | Музыка+речь |
| CELT (NB) | 32-64 kbps | 5 ms | Музыка/NB |
| CELT (FB) | 64-128 kbps | 2.5-5 ms | Музыка/FB |

---

## 9. H.264/VP8 — Математика видеокодека

### 9.1 Motion Estimation

```
// SAD (Sum of Absolute Differences):
SAD(mv) = Σ |curr_block(i,j) - ref_block(i+mv_x, j+mv_y)|

// SATD (Hadamard transform):
SATD(mv) = Σ |H · (curr - ref) · H^T|

// Rate-distortion optimized motion search:
J_motion = SAD/SATD + λ_motion · R(mv)
```

### 9.2 DCT (Discrete Cosine Transform) — H.264

```
// 4x4 Integer DCT:
Y(u,v) = (2/N) · Σ Σ C(u) · C(v) · y(x,y) · cos(...)

// H.264 integer approximation:
Y = C_f · X · C_f^T ⊗ E_f

где:
  C_f = [[1, 1, 1, 1],
         [2, 1, -1, -2],
         [1, -1, -1, 1],
         [1, -2, 2, -1]]

  E_f — матрица масштабирования
```

### 9.3 Quantization

```
// H.264:
Q(u,v) = round(Y(u,v) / Qstep)
Qstep ≈ 2^(QP/6)     // удваивается каждые 6 QP

// VP8:
Q = clamp(round(Y / DC_quant), 0, 127)

// Dead-zone для VP8:
if (|Y| < dead_zone):
    Q = 0
else:
    Q = sign(Y) * floor((|Y| - dead_zone) / Qstep)
```

### 9.4 Rate-Distortion Optimization

```
J = D + λ · R

где:
  D — искажение (SSD или SAD)
  R — битрейт
  λ = 0.85 · 2^(QP/3)  // H.264 Lagrangian multiplier

// Для Intra/Inter выбора:
J_intra = D_intra + λ_intra · R_intra
J_inter = D_inter + λ_inter · R_inter

// Выбираем min(J_intra, J_inter)
```

### 9.5 In-loop Deblocking Filter

```
// H.264 Deblocking:
BS ∈ {0, 1, 2, 3, 4}  // Boundary Strength

if (BS > 0):
    Δ = clamp((a3 - a2 * BS) >> 3, -c, c)
    p0' = clip(p0 + Δ)
    q0' = clip(q0 - Δ)
```

### 9.6 VP8/VP9 особенности

```
// VP8 DCT:
Y(u,v) = (1/√N) · C(u) · Σ y(n) · cos((2n+1)·uπ/2N)

// VP9 compound prediction:
prediction = (1 - α) · inter_pred + α · intra_pred
где α ∈ [0, 1] — вес inter/intra
```

---

## 10. Simulcast — Мультибитрейтная математика

### 10.1 Temporal Layers

```
FPS_layer = FPS_base / 2^(L - 1)

где:
  L — номер слоя (1 = base)
  FPS_base — базовый FPS (например, 30 fps)

Пример (3 temporal layers):
  Layer 1: 30 fps (все кадры)
  Layer 2: 15 fps (T0 + T2)
  Layer 3: 7.5 fps (только T0)
```

### 10.2 Spatial Layers

```
W_layer = W_base / 2^(L - 1)
H_layer = H_base / 2^(L - 1)

Пример:
  Layer 0: 1920×1080
  Layer 1: 960×540
  Layer 2: 480×270
```

### 10.3 Bitrate Allocation

```
B_L = B_total · w_L / Σw_i

где:
  w_L — вес слоя (пример: [0.5, 0.3, 0.2])
  B_total — общий доступный битрейт

// Adaptive bitrate per layer:
B_L = max(min_bitrate_L, min(max_bitrate_L, B_L))
```

### 10.4 SVC vs Simulcast

| Характеристика | Simulcast | SVC (VP9/SVC) |
|----------------|-----------|----------------|
| Независимость потоков | Полная | Частичная |
| Вычислительная нагрузка | 3× encode | 1.5× encode |
| Пропускная способность | ΣB_L | B_max_single ≈ 1.2× B_single |
| Клиентская гибкость | Выбор потока | Выбор слоя на лету |

---

## 11. Вероятность NAT Traversal

### 11.1 Pair Connectivity Probability

| Pair | Cone | Restricted Cone | Port Restricted | Symmetric |
|------|------|----------------|-----------------|-----------|
| **Cone** | 95% | 90% | 85% | 50% |
| **Restricted Cone** | 90% | 85% | 80% | 40% |
| **Port Restricted** | 85% | 80% | 70% | 30% |
| **Symmetric** | 50% | 40% | 30% | 0% (TURN) |

### 11.2 STUN Timeout Backoff

```
RTO_0 = 500ms
RTO_n = min(RTO_(n-1) * 2, 7500ms)   // max 7.5 sec

// Total timeout:
timeout_total = Σ RTO_n               // n = 0..6
timeout_total ≈ 500 + 1000 + 2000 + 4000 + 7500 + 7500 + 7500
              = 30 seconds

// Вероятность успеха после N попыток:
P_success(N) = 1 - (1 - P_single)^N
```

### 11.3 STUN Transaction Success

```
// С вероятностью p_success для одной попытки:
P(success | N стун запросов) = 1 - (1 - p_success)^N

// пример: p_success = 0.35 (Port Restricted → Port Restricted)
P(success | 3 попытки) = 1 - (1 - 0.35)^3 ≈ 0.725
P(success | 7 попыток) = 1 - (1 - 0.35)^7 ≈ 0.951
```

### 11.4 TURN Fallback

```
// Когда ICE не проходит:
if (ice_state == FAILED):
    // 100% гарантия через TURN (релейный сервер)
    bitrate = bitrate * 0.5   // overhead relay

// TURN bandwidth overhead:
B_eff = B · (1 - overhead)
overhead ≈ 0.1..0.3  // в зависимости от протокола (UDP/TCP/TLS)
```

### 11.5 ICE Nomination тайминги

```
// Regular nomination:
Nomination time ≈ RTT + 200ms (processing)

// Aggressive nomination:
Nomination прямо в connectivity check
Время ≈ RTT / 2 (speculative)

// Total ICE flow:
Total_ICE_time = STUN_gathering + connectivity_check + nomination
Total_ICE_time ≈ 500ms (host) .. 5-10s (relay)
```

---

## 12. Сжатие данных при передаче через DataChannel

При передаче изображений и видео через DataChannel (например, файлообмен между пользователями) сжатие на стороне отправителя существенно снижает трафик. **Встраивать ffmpeg для этого не рекомендуется** — вместо этого используются нативные браузерные API.

### 12.1 Сжатие изображений (Canvas API)

```
// Браузер:
img       → Canvas          → toBlob('image/webp', quality)
файл        отрисовка          сжатие в WebP/JPEG

// Результат:
размер_исх ≈ 5 MB (RAW/PNG)
размер_webp = compressed(Canvas, quality=0.8)
            ≈ 0.2..0.5 MB   // в 10-25 раз меньше

// Формула выигрыша:
ratio = 1 - size_compressed / size_original

// Качество можно адаптировать под битрейт канала:
if (bitrate < threshold):
    quality = max(0.1, quality - 0.1)
else:
    quality = min(1.0, quality + 0.05)
```

- Алгоритмическая сложность: O(W × H) — аппаратное ускорение браузера
- Никаких внешних зависимостей (не нужен ffmpeg.wasm)
- Работает во всех современных браузерах

### 12.2 Сжатие видео (MediaRecorder API)

```
// Запись/пережатие видео:
stream → MediaRecorder(codec='vp8', bitrate=bits_per_second)
         ↓
         chunks → DataChannel.send(chunk)

// Выбор битрейта:
bitrate = clamp(target_bitrate, min_bitrate, max_bitrate)

// Адаптация к пропускной способности канала:
if (packet_loss > 0.05):
    bitrate *= 0.8     // снижаем при потерях
elif (rtt < 100ms):
    bitrate *= 1.05   // повышаем при хорошем канале
```

- Кодек: VP8 (по умолчанию), можно VP9, H.264 (зависит от браузера)
- MediaRecorder использует встроенные кодеки браузера — без внешних зависимостей
- Размер сжатого видео: 500 KB ÷ 5 MB/min (зависит от битрейта и сцены)

### 12.3 ffmpeg.wasm — когда всё-таки нужен

```
ffmpeg.wasm ≈ 30 MB (загрузка)
             ⚠ не рекомендуется для мобильных устройств
             ✓ полезен для серверной перекодировки

// Случаи применения:
- Конвертация между контейнерами (MOV → MP4)
- Изменение FPS, разрешения
- Пакетная обработка
- Нарезка/склейка видео

// Альтернатива: серверное пережатие
Client → server (ffmpeg) → receiver
```

### 12.4 Сравнение подходов

| Подход | Размер библиотеки | Производительность | Поддержка форматов | Где применять |
|--------|------------------|--------------------|--------------------|---------------|
| Canvas + toBlob | 0 KB (встроено) | аппаратное ускорение | WebP, JPEG, PNG | Изображения |
| MediaRecorder | 0 KB (встроено) | аппаратное ускорение | VP8, VP9, H.264 | Видео/запись |
| ffmpeg.wasm | ~30 MB | WASM (CPU) | Все | Сервер/десктоп |
| Серверный ffmpeg | 0 KB (клиент) | выделенный сервер | Все | Серверная архитектура |

### 12.5 Рекомендация

```
Для веб-приложения:
  изображения → Canvas + toBlob('image/webp', 0.8)  ✓
  видео       → MediaRecorder({ mimeType: 'video/webm;codecs=vp9' }) ✓
  ffmpeg      → только на сервере (если нужен)       ✓

Выигрыш трафика:
  изображения: 10x-25x
  видео:       5x-20x (зависит от битрейта)
```

---

## Список литературы

1. RFC 5245 — Interactive Connectivity Establishment (ICE)
2. RFC 5389 — Session Traversal Utilities for NAT (STUN)
3. RFC 5766 — Traversal Using Relays around NAT (TURN)
4. RFC 3711 — Secure Real-time Transport Protocol (SRTP)
5. RFC 5764 — DTLS Extension to Establish Keys for SRTP
6. RFC 4960 — Stream Control Transmission Protocol
7. RFC 8830 — WebRTC Protocol
8. draft-ietf-rmcat-gcc-02 — Google Congestion Control
9. RFC 6716 — Opus Audio Codec
10. ITU-T H.264 — Advanced video coding
12. RFC 6386 — VP8 Data Format and Decoding Guide
