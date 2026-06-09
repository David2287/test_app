# Безопасность

## Модель угроз

| Угроза           | Описание                                              | Меры защиты                                       |
|------------------|-------------------------------------------------------|---------------------------------------------------|
| Прослушивание    | Атакующий в том же VLAN/LAN перехватывает трафик      | TLS шифрование между пирами                       |
| Имитация         | Атакующий выдает себя за известного пира              | TOFU + проверка отпечатка публичного ключа        |
| Повторная атака  | Перехваченные сообщения вбрасываются заново           | Timestamp + дедупликация по messageId             |
| Спуфинг обнаружения | Фальшивые heartbeat/mDNS объявления               | Внеполосная верификация пира                      |
| Сетевая изоляция | Пиры не должны общаться через недоверенные сети       | VLAN-границы создают естественную сегментацию     |

## Шифрование транспорта (TLS)

Каждый пир генерирует самоподписанный X.509 сертификат при первом запуске. Ключевая пара сертификата используется как для TLS-рукопожатия, так и для подписи сообщений.

```kotlin
fun generateSelfSignedCertificate(deviceId: String): KeyPair {
    val keyPairGenerator = KeyPairGenerator.getInstance("EC")
    keyPairGenerator.initialize(ECGenParameterSpec("secp256r1"))
    val keyPair = keyPairGenerator.generateKeyPair()

    val startDate = Date()
    val endDate = Date(startDate.time + 365 * 24 * 60 * 60 * 1000L)

    val certInfo = X509CertInfo().apply {
        set(X509CertInfo.VERSION, CertificateVersion(CertificateVersion.V3))
        set(X509CertInfo.SERIAL_NUMBER, BigInteger(64, SecureRandom()))
        set(X509CertInfo.ALGORITHM_ID, AlgorithmId(AlgorithmId.ecdsaWithSHA256_oid))
        set(X509CertInfo.SUBJECT, X500Name("CN=$deviceId"))
        set(X509CertInfo.ISSUER, X500Name("CN=$deviceId"))
        set(X509CertInfo.VALIDITY, CertificateValidity(startDate, endDate))
        set(X509CertInfo.KEY, CertificateX509Key(keyPair.public))
    }

    val cert = CertificateFactory.getInstance("X.509")
        .generateCertificate(ByteArrayInputStream(certInfo.encoded))
    return keyPair
}
```

## Доверие при первом контакте (TOFU)

При первом контакте с пиром:

1. Получаем его публичный ключ во время рукопожатия.
2. Вычисляем `SHA-256` отпечаток ключа.
3. Показываем отпечаток пользователю (hex строка или QR-код).
4. Пользователь проверяет внеполосно (вслух, сканирует QR).
5. При последующих соединениях отклонять, если отпечаток изменился.

```kotlin
data class PeerIdentity(
    val deviceId: String,
    val pubKeyFingerprint: String,
    val firstSeen: Long,
    val verified: Boolean
)

fun verifyPeerIdentity(
    stored: PeerIdentity?,
    receivedFingerprint: String
): VerificationResult {
    if (stored == null) {
        return VerificationResult.NewPeer(receivedFingerprint)
    }
    return if (stored.pubKeyFingerprint == receivedFingerprint) {
        VerificationResult.Match
    } else {
        VerificationResult.Mismatch
    }
}
```

## Конфиденциальность сообщений

После TLS-рукопожатия все сообщения шифруются при передаче. Прикладной слой также поддерживает **сквозное шифрование (E2EE)** (опционально):

- Каждый пир хранит пару ключей Ed25519 (отдельно от TLS-сертификата).
- Текст сообщения шифруется с помощью `libsodium` или `java.security` публичным ключом получателя.
- E2EE по умолчанию отключён для простоты, но протокол поддерживает поле `encryptedPayload`:

```json
{
  "type": "TEXT",
  "encrypted": true,
  "nonce": "base64...",
  "senderPubKey": "base64...",
  "ciphertext": "base64..."
}
```

## Дополнительные меры

| Мера                     | Реализация                                        |
|--------------------------|---------------------------------------------------|
| Оборация порта           | Динамический порт на сессию (нефиксированный)     |
| Ограничение скорости     | Макс. 100 сообщений/мин на пира                   |
| Лимит размера нагрузки   | Отклонять фреймы > 10 МБ                          |
| Таймаут соединения       | 30с бездействия → закрыть сокет                   |
| Очистка логов            | Никогда не логировать содержимое сообщений         |
| Локальное хранение       | Room БД зашифрована через `EncryptedRoomDatabase` (SQLCipher) |
