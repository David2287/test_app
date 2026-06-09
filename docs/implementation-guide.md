# Руководство по реализации

## Структура проекта

```
app/
├── build.gradle.kts
├── src/
│   └── main/
│       ├── AndroidManifest.xml
│       ├── java/com/p2pmsg/
│       │   ├── App.kt
│       │   ├── discovery/
│       │   │   ├── DiscoveryEngine.kt
│       │   │   ├── MdnsDiscovery.kt
│       │   │   └── UdpHeartbeat.kt
│       │   ├── transport/
│       │   │   ├── ConnectionManager.kt
│       │   │   ├── TcpServer.kt
│       │   │   ├── TcpClient.kt
│       │   │   └── TlsWrapper.kt
│       │   ├── protocol/
│       │   │   ├── Frame.kt
│       │   │   ├── MessageSerializer.kt
│       │   │   └── HandshakeHandler.kt
│       │   ├── service/
│       │   │   ├── P2PSessionManager.kt
│       │   │   ├── RosterManager.kt
│       │   │   └── MessageRouter.kt
│       │   ├── data/
│       │   │   ├── local/
│       │   │   │   ├── AppDatabase.kt
│       │   │   │   ├── MessageDao.kt
│       │   │   │   └── RosterDao.kt
│       │   │   └── model/
│       │   │       ├── Message.kt
│       │   │       └── Peer.kt
│       │   └── ui/
│       │       ├── MainActivity.kt
│       │       ├── chat/
│       │       └── contacts/
│       └── res/
```

## build.gradle.kts

```kotlin
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.serialization")
    id("com.google.devtools.ksp") // для Room
}

android {
    namespace = "com.p2pmsg"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.p2pmsg"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"
    }

    buildFeatures {
        compose = true
    }

    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.10"
    }
}

dependencies {
    // Compose
    implementation(platform("androidx.compose:compose-bom:2024.05.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.activity:activity-compose:1.9.0")

    // Сериализация
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.3")

    // Coroutines
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    // Room
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")

    // Криптография
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // Lifecycle
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.7.0")
}
```

## Реализация обнаружения

### mDNS (Android NSD)

```kotlin
class MdnsDiscovery(
    private val context: Context,
    private val onPeerFound: (Peer) -> Unit
) {
    private val nsdManager = context.getSystemService(Context.NSD_SERVICE) as NsdManager
    private var serviceName: String? = null

    fun registerService(localPort: Int, deviceId: String, peerName: String) {
        val txtRecords = mapOf("peerName" to peerName, "version" to "1")
        val serviceInfo = NsdServiceInfo().apply {
            serviceType = "_p2pmsg._tcp"
            serviceName = "P2PMsg-$deviceId"
            port = localPort
            txtRecords.forEach { (k, v) -> setAttribute(k, v) }
        }
        nsdManager.registerService(serviceInfo, NsdManager.PROTOCOL_DNS_SD, object : NsdManager.RegistrationListener {
            override fun onServiceRegistered(info: NsdServiceInfo) {
                serviceName = info.serviceName
            }
            override fun onRegistrationFailed(info: NsdServiceInfo, errorCode: Int) {}
            override fun onServiceUnregistered(info: NsdServiceInfo) {}
            override fun onUnregistrationFailed(info: NsdServiceInfo, errorCode: Int) {}
        })
    }

    fun discoverServices() {
        nsdManager.discoverServices("_p2pmsg._tcp", NsdManager.PROTOCOL_DNS_SD, object : NsdManager.DiscoveryListener {
            override fun onServiceFound(info: NsdServiceInfo) {
                nsdManager.resolveService(info, object : NsdManager.ResolveListener {
                    override fun onServiceResolved(info: NsdServiceInfo) {
                        val peer = Peer(
                            deviceId = info.serviceName.removePrefix("P2PMsg-"),
                            displayName = info.attributes["peerName"]?.toString() ?: "Unknown",
                            host = info.host.hostAddress,
                            port = info.port
                        )
                        onPeerFound(peer)
                    }
                    override fun onResolveFailed(info: NsdServiceInfo, errorCode: Int) {}
                })
            }
            override fun onServiceLost(info: NsdServiceInfo) {}
            override fun onDiscoveryStarted(serviceType: String) {}
            override fun onDiscoveryStopped(serviceType: String) {}
            override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {}
            override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {}
        })
    }
}
```

### UDP Heartbeat

```kotlin
class UdpHeartbeat(
    private val config: PeerConfig,
    private val onHeartbeatReceived: (Peer) -> Unit
) {
    private val socket = MulticastSocket(BROADCAST_PORT).apply {
        broadcast = true
        reuseAddress = true
        timeToLive = 2
    }
    private val sendPacket = DatagramPacket(
        ByteArray(MAX_PACKET_SIZE), MAX_PACKET_SIZE,
        InetAddress.getByName("255.255.255.255"), BROADCAST_PORT
    )

    fun start() {
        CoroutineScope(Dispatchers.IO).launch {
            // Цикл приёма
            while (isActive) {
                val buf = ByteArray(MAX_PACKET_SIZE)
                val packet = DatagramPacket(buf, buf.size)
                socket.receive(packet)
                val json = String(packet.data, 0, packet.length)
                val peer = parseHeartbeat(json) ?: continue
                if (peer.deviceId != config.deviceId) {
                    onHeartbeatReceived(peer)
                }
            }
        }

        // Цикл отправки
        CoroutineScope(Dispatchers.IO).launch {
            while (isActive) {
                val payload = buildHeartbeatJson(config)
                sendPacket.data = payload.toByteArray()
                sendPacket.length = payload.length
                socket.send(sendPacket)
                delay(5000)
            }
        }
    }
}
```

## TCP транспорт

### Фрейминг (чтение/запись)

```kotlin
object Frame {
    suspend fun readFrame(input: InputStream): Frame? {
        val lenBuf = ByteArray(4)
        if (input.readExact(lenBuf) != 4) return null
        val length = (lenBuf[0].toInt() and 0xFF shl 24) or
                     (lenBuf[1].toInt() and 0xFF shl 16) or
                     (lenBuf[2].toInt() and 0xFF shl 8) or
                     (lenBuf[3].toInt() and 0xFF)
        val type = input.read()
        if (type == -1) return null
        val payload = ByteArray(length - 1)
        input.readExact(payload)
        return Frame(type.toByte(), payload)
    }

    suspend fun writeFrame(output: OutputStream, type: Byte, payload: ByteArray) {
        val length = 1 + payload.size
        val header = byteArrayOf(
            (length shr 24).toByte(),
            (length shr 16).toByte(),
            (length shr 8).toByte(),
            length.toByte(),
            type
        )
        output.write(header)
        output.write(payload)
        output.flush()
    }
}
```

## Жизненный цикл (Foreground Service)

Поиск пиров и управление соединениями должны работать в **foreground-сервисе**, чтобы переживать уничтожение Activity.

```kotlin
class P2pForegroundService : Service() {
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("P2P Messenger")
            .setContentText("Поиск пиров...")
            .setSmallIcon(R.drawable.ic_notification)
            .build()
        startForeground(NOTIFICATION_ID, notification)
        // Запуск discovery engine, TCP сервера
        return START_STICKY
    }
}
```

Регистрация в `AndroidManifest.xml`:

```xml
<service
    android:name=".service.P2pForegroundService"
    android:foregroundServiceType="dataSync"
    android:exported="false" />
```

## Важные замечания по реализации

- **Изменения сети:** Зарегистрировать `ConnectivityManager.NetworkCallback` для перезапуска обнаружения при переподключении WiFi или изменении IP.
- **Wake lock:** Получить частичный wake lock через `PowerManager`, чтобы heartbeat-отправитель работал в режиме сна.
- **Оптимизация батареи:** Запросить `REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` для надёжной фоновой работы.
- **NAT:** Не применимо, так как приложение работает исключительно в локальных сетях без NAT traversal.
