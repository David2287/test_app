# Procedure Memory — p2p-node → p2p_messenger

## Architecture (Two Projects)

```
┌───────────────────────────────────────────────────────────────────────────┐
│  p2p-node/  (Go)                                                          │
│  ┌─────────┐  ┌─────────┐  ┌────────┐  ┌──────────┐  ┌──────────┐       │
│  │ node.go │  │ dht.go  │  │mdns.go │  │ relay.go │  │signal.go │       │
│  └────┬────┘  └────┬────┘  └───┬────┘  └────┬─────┘  └────┬─────┘       │
│       └────────────┴──────────┴─────────────┴─────────────┘              │
│                                ↓              ┌──────────────┐          │
│                     ┌───────────────────┐     │ crypto/e2ee │          │
│                     │ bridge/mobilenode │ ←──│ X25519+ChaCha│          │
│                     │   (MobileNode)    │     └──────────────┘          │
│                     └─────────┬─────────┘                                │
└───────────────────────────────┼────────────────────────────────────────┘
                                │ Platform Channel (MethodChannel)
┌───────────────────────────────┼────────────────────────────────────────┐
│  p2p_messenger/  (Flutter/Dart)                                        │
│                     ┌───────────────────┐   ┌──────────────────┐       │
│                     │  P2pBridgeChannel │   │  E2eeService     │       │
│                     │  or P2pBridgeMock │   │ (Bridge|Mock)    │       │
│                     └─────────┬─────────┘   └────────┬─────────┘       │
│                               ↓                      ↓                  │
│                     ┌──────────────────────────────────────┐           │
│                     │     PeerConnectionManager            │           │
│                     │  (SDP/ICE + E2EE encrypt/decrypt)    │           │
│                     └──────────────────┬───────────────────┘           │
│                    ┌───────────────────┴────────────────┐              │
│                    │       main.dart (E2EE UI)          │              │
│                    └────────────────────────────────────┘              │
└────────────────────────────────────────────────────────────────────────┘
```

## Project Structure

### p2p-node/ (Go)

```
p2p-node/
  go.mod                           # module github.com/whyskydie/p2p-node, Go 1.26.2
  go.sum
  docs/
    procedure_memory.md            ← this file
  p2p/
    node.go                        # Node — libp2p host wrapper
    dht.go                         # DHT — Kademlia routing wrapper
    mdns.go                        # Discovery — mDNS LAN discovery wrapper
    relay.go                       # Relay — Circuit Relay v2 wrapper
    signaling.go                   # Signaling — SDP/ICE exchange over libp2p streams
    tests/
      node_test.go                 # 6 tests (Node)
      dht_test.go                  # 6 tests (DHT)
      mdns_test.go                 # 4 tests (mDNS Discovery, 1 skip on non-Linux)
      relay_test.go                # 3 tests (Circuit Relay v2)
      signaling_test.go            # 3 tests (Signaling protocol)
  bridge/
    mobilenode.go                  # gomobile bridge — MobileNode + EventHandler
    tests/
      mobilenode_test.go           # 16 tests (incl. E2EE via signaling)
  cmd/mobile/main.go               # gomobile entry point (empty)
  crypto/
    e2ee.go                        # X25519 + ChaCha20-Poly1305
    e2ee_test.go                   # 11 tests
```

**Total: 43 Go tests, all pass.** Run: `go test ./... -v -count=1 -timeout 60s`

**Test packages:** `p2p/tests/` (22 tests) + `bridge/tests/` (16 tests) + `crypto/` (11 tests).

### p2p_messenger/ (Flutter/Dart)

```
p2p_messenger/
  pubspec.yaml                     # Flutter 3.41.9, Dart 3.11.5
  lib/
    main.dart                      # Entry point (P2pApp → HomeScreen)
    models/
      p2p_event.dart                # P2pEvent model (type, peerId, sessionId, signalType, signalData, addrs, message)
    services/
      p2p_bridge_service.dart      # P2pBridgeService (abstract) + P2pBridgeChannel (MethodChannel/EventChannel)
      p2p_bridge_mock.dart         # P2pBridgeMock (dev-mode without Go bridge)
      e2ee_service.dart            # E2eeService (abstract) + E2eeBridgeService + E2eeMockService
    webrtc/
      peer_connection_manager.dart # RTCPeerConnection per session, SDP/ICE via signaling, DataChannel
  android/
    app/
      build.gradle.kts             # minSdk = 24
      libs/                        # <-- p2p-node.aar (to be copied after gomobile build)
      src/main/kotlin/com/whyskydie/p2p_messenger/
        MainActivity.kt            # Registers P2pBridgePlugin
        P2pBridgePlugin.kt         # MethodChannel → Kotlin → Go MobileNode (reflection)
  ios/
    Runner/
      AppDelegate.swift            # Registers P2pBridgePlugin
      P2pBridgePlugin.swift        # MethodChannel → Swift → Go MobileNode (direct import)
  test/
    widget_test.dart               # Smoke test (App renders with Connect/Call buttons)
```

**Flutter lint + test:** `flutter analyze` → clean, `flutter test` → 1/1 passes.

---

## Dependencies

### Go (go.mod direct)

| Package | Version | Used in |
|---|---|---|
| `github.com/libp2p/go-libp2p` | v0.48.0 | node, dht, relay, signaling |
| `github.com/libp2p/go-libp2p-kad-dht` | v0.40.0 | dht |
| `github.com/multiformats/go-multiaddr` | v0.16.1 | node, dht, relay, bridge |
| `github.com/multiformats/go-multihash` | v0.2.3 | dht (CID generation) |
| `github.com/ipfs/go-cid` | v0.6.1 | dht (CID from string key) |
| `github.com/stretchr/testify` | v1.11.1 | all tests |
| `golang.org/x/mobile` | latest (tool) | gomobile bind → .aar/.xcframework |
| `golang.org/x/crypto` | v0.50.0 (indirect) | crypto/e2ee (curve25519 + chacha20poly1305) |

### Flutter (pubspec.yaml)

| Package | Version | Used in |
|---|---|---|
| `flutter` | SDK 3.41.9 | main, services, webrtc |
| `flutter_webrtc` | 1.4.1 | PeerConnectionManager |
| `dart_webrtc` | 1.8.1 (transitive) | PeerConnectionManager (RTCSessionDescription, RTCIceCandidate, RTCPeerConnection) |
| `webrtc_interface` | 1.5.1 (transitive) | PeerConnectionManager (no RTCIceServer type — use Map<String,dynamic> config) |

---

## File-by-File API Reference

### `p2p/node.go` — Node (libp2p host)

**Types:**

| Type | Kind | Description |
|---|---|---|
| `Node` | struct | libp2p host + peerID + context |
| `Config` | struct | Port, PrivateKey, ListenAddrs, ConnMgrLow/High |
| `Option` | `func(*Config)` | functional options pattern |

**Node fields** (unexported):
- `host host.Host` — underlying libp2p host
- `peerID peer.ID` — cached peer ID
- `ctx context.Context` — background context
- `cancel context.CancelFunc` — for cleanup

**Functions:**

| Signature | Description |
|---|---|
| `WithPort(port int) Option` | set TCP listen port (0 = random) |
| `WithPrivateKey(key crypto.PrivKey) Option` | deterministic peer ID |
| `WithListenAddrs(addrs ...string) Option` | override listen address |
| `WithConnectionManager(low, high int) Option` | enable connmgr |
| `NewNode(opts ...Option) (*Node, error)` | create node |
| `ListenPort(h host.Host) (int, error)` | extract TCP port from host addrs |

**Methods on `*Node`:**

| Signature | Description |
|---|---|
| `PeerID() peer.ID` | get peer ID |
| `Host() host.Host` | get underlying libp2p host |
| `Multiaddrs() []multiaddr.Multiaddr` | get `/ip4/.../tcp/.../p2p/<peerID>` addrs |
| `Close() error` | cancel context + close host |
| `Connect(addr multiaddr.Multiaddr) error` | parse p2p addr, add to peerstore, dial |

**Key libp2p patterns:**
- `libp2p.New(opts...)` creates host
- `libp2p.Identity(key)` for deterministic identity
- `libp2p.ListenAddrStrings(addr)` for listen
- `connmgr.NewConnManager(low, high)` for connection manager
- `peer.AddrInfoFromP2pAddr(addr)` parses `/ip4/.../tcp/.../p2p/<id>` to AddrInfo
- `peerstore.PermanentAddrTTL` for address TTL
- Multiaddrs format: `/ip4/<ip>/tcp/<port>/p2p/<peerID>`

---

### `p2p/dht.go` — DHT (Kademlia Routing)

**Types:**

| Type | Kind | Description |
|---|---|---|
| `DHTMode` | int (iota) | DHTModeClient or DHTModeServer |
| `DHTConfig` | struct | Mode, BootstrapPeers |
| `DHTOption` | `func(*DHTConfig)` | functional options |
| `DHT` | struct | Kademlia DHT wrapper |

**DHT fields** (unexported):
- `host host.Host`
- `dht *kaddht.IpfsDHT`
- `config DHTConfig`
- `once sync.Once` — for Close()
- `closer func()`

**Constants:**

| Name | Value |
|---|---|
| `DHTModeClient` | 0 |
| `DHTModeServer` | 1 |
| `peerstoreAddrTTL` | `time.Hour` (unexported) |

**Functions:**

| Signature | Description |
|---|---|
| `WithDHTMode(mode DHTMode) DHTOption` | set client/server mode |
| `WithBootstrapPeers(addrs []multiaddr.Multiaddr) DHTOption` | set bootstrap peers |
| `NewDHT(n *Node, opts ...DHTOption) (*DHT, error)` | create DHT |
| `cidFromString(s string) (cid.Cid, error)` | string key → CID v1 (Raw, SHA2-256) |

**Methods on `*DHT`:**

| Signature | Description |
|---|---|
| `Bootstrap(ctx context.Context) error` | connect bootstrap peers, start DHT (error if no peers) |
| `FindPeer(ctx context.Context, pid peer.ID) (peer.AddrInfo, error)` | find peer via DHT |
| `Provide(ctx context.Context, key string) error` | advertise as provider for key (string→SHA256→CID) |
| `FindProviders(ctx context.Context, key string) ([]peer.AddrInfo, error)` | find providers for key (10s timeout) |
| `WaitForPeer(ctx context.Context, pid peer.ID) error` | poll routing table until peer found (5ms intervals) |
| `Close() error` | close DHT (once) |

**CID generation** (`cidFromString`):
```
CID = CIDv1(Raw, SHA2_256, sum([]byte(key)))
```

**Routing table polling** (`WaitForPeer`):
- Uses `dht.RoutingTable().Find(pid)` — returns `peer.ID` as string or `""` if not found
- Polls every 5ms until context done

---

### `p2p/mdns.go` — Discovery (mDNS)

**Types:**

| Type | Kind | Description |
|---|---|---|
| `Notifee` | `= mdns.Notifee` | type alias for `github.com/libp2p/go-libp2p/p2p/discovery/mdns` |
| `Discovery` | struct | mDNS discovery wrapper |

**Discovery fields:**

| Field | Exported | Type | Description |
|---|---|---|---|
| `node` | no | `*Node` | owner node |
| `Service` | yes | `mdns.Service` | underlying mDNS service |
| `Notifee` | yes | `Notifee` | handler for discovered peers |
| `once` | no | `sync.Once` | ensures Close is idempotent |

**Functions:**

| Signature | Description |
|---|---|
| `NewDiscovery(node *Node, serviceName string, notifee Notifee) *Discovery` | create mDNS discovery (empty serviceName = `mdns.ServiceName`) |

**Methods on `*Discovery`:**

| Signature | Description |
|---|---|
| `Start() error` | start mDNS service |
| `Close() error` | stop mDNS service (idempotent via once) |

**Note:** mDNS `Notifee` interface has `HandlePeerFound(peer.AddrInfo)` method.

---

### `p2p/relay.go` — Relay (Circuit Relay v2)

**Types:**

| Type | Kind | Description |
|---|---|---|
| `Relay` | struct | circuit relay v2 wrapper |

**Relay fields** (unexported):
- `relay *circuit.Relay` from `go-libp2p/p2p/protocol/circuitv2/relay`

**Functions:**

| Signature | Description |
|---|---|
| `NewRelay(node *Node) (*Relay, error)` | create circuit relay on node |
| `CircuitAddr(relayP2pAddr multiaddr.Multiaddr, destID peer.ID) multiaddr.Multiaddr` | build `/p2p-circuit/p2p/<destID>` addr encapsulated in relay addr |
| `ReserveRelay(ctx context.Context, node *Node, relayP2pAddr multiaddr.Multiaddr) (*peer.AddrInfo, error)` | reserve slot on relay, return circuit addr for node |

**Methods on `*Relay`:**

| Signature | Description |
|---|---|
| `Close() error` | close relay |

**Circuit addr format:**
```
<relayP2pAddr>/p2p-circuit/p2p/<destPeerID>
```
Example: `/ip4/192.168.1.5/tcp/9000/p2p/QmRelay/p2p-circuit/p2p/QmDest`

**Key libp2p patterns:**
- `relay.New(host)` creates relay hop
- `client.Reserve(ctx, host, relayAddrInfo)` reserves slot
- `network.WithAllowLimitedConn(ctx, reason)` required when streaming via relay
- `Host().NewStream(ctx, peerID, protoID)` to open stream via relay

---

### `p2p/signaling.go` — Signaling (SDP/ICE exchange)

**Types:**

| Type | Kind | Description |
|---|---|---|
| `SignalMessageType` | string | "offer", "answer", or "ice-candidate" |
| `SignalMessage` | struct | Type + Data (JSON) |
| `SignalSession` | struct | active signaling session (stream) |
| `Signaling` | struct | signaling protocol handler |

**SignalSession fields** (unexported):
- `stream network.Stream`
- `enc *json.Encoder`
- `dec *json.Decoder`
- `remote peer.ID`
- `mu sync.Mutex`

**SignalSession methods:**

| Signature | Description |
|---|---|
| `Remote() peer.ID` | remote peer ID |
| `Send(ctx context.Context, msg SignalMessage) error` | send JSON message (with ctx cancellation, mutex-protected) |
| `Receive(ctx context.Context) (SignalMessage, error)` | receive JSON message (with ctx timeout, goroutine + channel) |
| `Close() error` | close stream |

**Signaling fields** (unexported):
- `node *Node`
- `handler func(*SignalSession)`
- `mu sync.Mutex`
- `started bool`

**Constants:**

| Name | Value |
|---|---|
| `signalingProtocol` | `protocol.ID("/p2p/signaling/1.0.0")` (unexported) |
| `SignalOffer` | `"offer"` |
| `SignalAnswer` | `"answer"` |
| `SignalICECandidate` | `"ice-candidate"` |

**Functions:**

| Signature | Description |
|---|---|
| `NewSignaling(node *Node) *Signaling` | create signaling handler |
| `newSignalSession(stream network.Stream) *SignalSession` | create session from stream (unexported) |

**Methods on `*Signaling`:**

| Signature | Description |
|---|---|
| `SetHandler(fn func(*SignalSession))` | set handler for incoming sessions |
| `Start() error` | register stream handler (idempotent) |
| `Dial(ctx context.Context, remote peer.ID) (*SignalSession, error)` | open signaling session to remote |
| `Close()` | remove stream handler (idempotent) |

**Message format (JSON):**
```json
{"type":"offer","data":"v=0\r\no=... sdp content"}
{"type":"answer","data":"v=0\r\no=... sdp content"}
{"type":"ice-candidate","data":"candidate:1 1 UDP 2122252543 192.168.1.5 5456 typ host"}
```

**Receive implementation pattern:** spawns goroutine for `dec.Decode`, uses channel + select for context cancellation.

### `bridge/mobilenode.go` — gomobile Bridge

**Constraint:** gomobile bind requires all exported types/methods to be simple types (no channels, no interfaces with unexported methods). Everything must be in package `bridge`.

**Types:**

| Type | Kind | Description |
|---|---|---|
| `EventHandler` | interface | `OnPeerDiscovered`, `OnSignalSession`, `OnSignalMessage`, `OnSignalSessionClosed`, `OnError` (verbose for mobile-friendly callbacks) |
| `MobileNode` | struct | wraps p2p.Node, p2p.DHT, p2p.Discovery, p2p.Relay, p2p.Signaling + crypto.KeyPair |

**MobileNode fields** (unexported):
- `node *p2p.Node`
- `dht *p2p.DHT`
- `disc *p2p.Discovery`
- `relay *p2p.Relay`
- `sig *p2p.Signaling`
- `handler EventHandler`
- `sessions map[string]*p2p.SignalSession`
- `ctx context.Context`
- `cancel context.CancelFunc`
- `seq int` — session counter
- `e2eeKey *crypto.KeyPair` — E2EE key pair
- `sharedSec map[string][]byte` — per-peer shared secrets

**Constructor:**

| Signature | Description |
|---|---|
| `NewMobileNode(handler EventHandler) *MobileNode` | create MobileNode with event callback |

**Methods on `*MobileNode` (all exported, native Go types — gomobile auto-converts to Java/Swift):**

| Signature | Return | Description |
|---|---|---|
| `PeerID() string` | peer ID | get own peer ID |
| `Addresses() []*Multiaddr` | addr list | get all multiaddrs |
| `AddressesString() []string` | addr strings | get multiaddrs as strings |
| `Connect(addr string) error` | — | connect to peer |
| `Disconnect(peerID string) error` | — | disconnect peer |
| `StartDHT(bootstrapPeers []string) error` | — | setup DHT (empty = client mode) |
| `FindPeer(peerID string) ([]string, error)` | addr list | find peer via DHT |
| `WaitForPeer(peerID string) error` | — | poll DHT until peer found |
| `Provide(key string) error` | — | advertise as provider |
| `FindProviders(key string) ([]string, error)` | peer ID list | find providers for key |
| `StartDiscovery(serviceName string) error` | — | start mDNS discovery |
| `StopDiscovery() error` | — | stop mDNS |
| `StartRelay() error` | — | enable circuit relay |
| `ReserveRelay(relayAddr string) (string, error)` | circuit addr | reserve relay slot |
| `StartSignaling() error` | — | register signaling handler |
| `DialSignal(peerID string) (string, error)` | session ID | open signaling session |
| `SendSignal(sessionID, msgType, data string) error` | — | send signal message |
| `CloseSignalSession(sessionID string) error` | — | close signaling session |
| `GenerateE2EEKey() (string, error)` | pub key hex | generate E2EE key pair |
| `SetRemoteKey(peerID, pubKeyHex string) error` | — | store peer's public key |
| `EncryptMessage(peerID string, plaintext []byte) (*EncryptResult, error)` | ciphertext hex | encrypt for peer |
| `DecryptMessage(peerID, ciphertextHex string) (*DecryptResult, error)` | plaintext bytes | decrypt from peer |
| `HasSharedSecret(peerID string) bool` | — | check E2EE readiness |
| `Close() error` | — | cleanup all resources |

**Event Handler interface** (verbose methods for mobile callbacks):

| Method | Parameters | Fired when |
|---|---|---|
| `OnPeerDiscovered` | `peerID, addrs` | mDNS discovered peer |
| `OnSignalSession` | `sessionID, peerID` | remote peer dialed signaling |
| `OnSignalMessage` | `sessionID, msgType, data` | SDP/ICE received |
| `OnSignalSessionClosed` | `sessionID` | signaling session ended |
| `OnError` | `message` | internal error |

**Key gomobile patterns:**
- gomobile handles Go types natively: `string`, `[]string`, `[]*T`, `error`, `bool` — all auto-converted to Java/Swift
- `EventHadler` is an exported interface with exported methods — gomobile generates callback stubs
- gomobile bind command: `gomobile bind -target=android -o <output>.aar ./bridge` (from p2p-node root)
- gomobile _requires_ Android NDK + Linux/macOS; does not work on Windows
- Reflection in Kotlin (`Class.forName("bridge.MobileNode")`) allows compiling without .aar

---

## Flutter Layer

### `lib/models/p2p_event.dart` — P2pEvent

**Fields:**

| Field | Type | Description |
|---|---|---|
| `type` | `String` | event type (peer_found, incoming_session, signal_message, etc.) |
| `peerId` | `String?` | remote peer ID |
| `sessionId` | `String?` | signaling session ID |
| `signalType` | `String?` | offer/answer/ice-candidate |
| `signalData` | `String?` | SDP or ICE candidate JSON |
| `addrs` | `List<String>?` | peer multiaddrs |
| `message` | `String?` | direct message content |

**Methods:**
- `P2pEvent.fromMap(Map<String, dynamic> map)` — parse from EventChannel JSON
- `toMap()` — serialize to Map

### `lib/services/p2p_bridge_service.dart` — P2pBridgeService + P2pBridgeChannel

**Abstract class `P2pBridgeService`:**

| Method | Returns | Description |
|---|---|---|
| `createNode({int? port, String? keyHex})` | `Future<String>` | create node, returns peer ID |
| `startMDNS({String? serviceName})` | `Future<void>` | start mDNS discovery |
| `setupDHT(int mode, String bootstrapJSON)` | `Future<void>` | setup DHT 0=client, 1=server |
| `startSignaling()` | `Future<void>` | register signaling handler |
| `enableRelay()` | `Future<void>` | create circuit relay |
| `connect(String addr)` | `Future<void>` | connect to peer by multiaddr |
| `provide(String key)` | `Future<void>` | advertise as provider |
| `findProviders(String key)` | `Future<String>` | find providers for key |
| `reserveRelay(String relayAddr)` | `Future<String>` | reserve relay slot |
| `dialSignaling(String peerID)` | `Future<String>` | open signaling session |
| `sendSignal(String sessionID, String type, String data)` | `Future<void>` | send SDP/ICE |
| `closeSession(String sessionID)` | `Future<void>` | close signaling session |
| `sendMessage(String peerID, String message)` | `Future<void>` | send direct message |
| `getPeerID()` | `Future<String>` | get own peer ID |
| `getMultiaddrs()` | `Future<String>` | get own multiaddrs |
| `nodeInfo()` | `Future<String>` | get full node info |
| `close()` | `Future<void>` | cleanup |
| `get events` | `Stream<P2pEvent>` | event stream from EventChannel |

**Concrete class `P2pBridgeChannel` implements `P2pBridgeService`:**
- Uses `MethodChannel('p2p_bridge')` — each method invokes `invokeMethod(...)` with the same name, arguments as Map
- Uses `EventChannel('p2p_events')` — emits `P2pEvent` from JSON strings via `P2pEvent.fromMap`

### `lib/services/p2p_bridge_mock.dart` — P2pBridgeMock (Dev Mode)

Implements `P2pBridgeService` for Windows/dev without Go bridge. Generates fake peer IDs (`12D3KooWMock<epoch>`). Simulates events via `StreamController<P2pEvent>.broadcast()`.

### `lib/webrtc/peer_connection_manager.dart` — PeerConnectionManager

**Architecture:**
```
P2pBridgeService.events
  └─ signal_message ─→ PeerConnectionManager
                        ├─ session_A ← RTCPeerConnection
                        ├─ session_B ← RTCPeerConnection
                        └─ session_N ← RTCPeerConnection
```

**Internal type `_Session`:**
- `sessionId` — session ID string
- `pc` — `RTCPeerConnection`
- `pendingCandidates` — buffered `RTCIceCandidate`s
- `dataChannel` — `RTCDataChannel?`

**Fields:**

| Field | Type | Description |
|---|---|---|
| `_bridge` | `P2pBridgeService` | injected bridge service |
| `_sessions` | `Map<String, _Session>` | active sessions |
| `_onDataMessage` | `void Function(String, String)?` | callback for DataChannel messages |
| `onDataMessage` setter | | for external listeners |

**Methods:**

| Signature | Description |
|---|---|
| `Future<String> createOffer(String sessionID)` | create offer, set local desc, send via bridge, return sessionID |
| `Future<void> createAnswer(String sessionID, String sdp)` | create answer from offer, set local desc, send via bridge |
| `Future<void> handleSignal(String sessionID, String type, String data)` | route SDP/ICE to correct handler |
| `Future<void> _addIceCandidate(String sessionID, String candidate)` | add ICE candidate to connection |
| `void _closeSession(String sessionID)` | close RTCPeerConnection, clean up |
| `void dispose()` | close all sessions |

**WebRTC Configuration (no RTCIceServer type):**
```dart
final config = <String, dynamic>{
  'iceServers': <Map<String, dynamic>>[
    {'urls': 'stun:stun.l.google.com:19302'},
  ],
};
final pc = await createPeerConnection(config);
```

**DataChannel handling:**
- `onDataChannel` callback set on each `RTCPeerConnection`
- Creates `RTCDataChannelMessage` listener → `_onDataMessage(sessionId, text)`

### `lib/main.dart` — Entry Point

- `P2pApp` (StatelessWidget) — MaterialApp with `HomeScreen`
- `HomeScreen` (StatefulWidget) — build/toggle buttons: Connect, Call, Send Message
- Uses `P2pBridgeMock` for dev, ready to switch to `P2pBridgeChannel` for production
- Console-style log display with timestamps

## E2EE (End-to-End Encryption)

### Go — `crypto/e2ee.go`

**Algorithm:** X25519 key exchange + ChaCha20-Poly1305 (XChacha20 variant with 24-byte nonce)

**Types:**

| Type | Description |
|---|---|
| `KeyPair` | struct with `PrivateKey [32]byte` and `PublicKey [32]byte` |

**Functions:**

| Signature | Description |
|---|---|
| `GenerateKey() (*KeyPair, error)` | generate random X25519 key pair |
| `SharedSecret(priv, pub [32]byte) []byte` | derive 32-byte shared secret |
| `Encrypt(sharedSecret, plaintext []byte) ([]byte, error)` | seal with random nonce prepended |
| `Decrypt(sharedSecret, ciphertext []byte) ([]byte, error)` | open AEAD, returns `ErrDecrypt` on failure |
| `(kp *KeyPair) Marshal() (privHex, pubHex string)` | keys to hex |
| `UnmarshalKey(privHex, pubHex string) (*KeyPair, error)` | hex to KeyPair |

**Constants / Errors:**
- `NonceSize = 24` (chacha20poly1305.NonceSizeX)
- `ErrDecrypt` — returned on auth failure or tampered ciphertext

**Ciphertext format:** `[24-byte nonce][encrypted payload][16-byte MAC]`

### Bridge — MobileNode E2EE methods

Added to `bridge/mobilenode.go`:

| Method | Returns | Description |
|---|---|---|
| `GenerateE2EEKey() (string, error)` | public key hex | generate and store local key pair |
| `SetRemoteKey(peerID, pubKeyHex string) error` | — | store peer key, derive shared secret |
| `EncryptMessage(peerID string, plaintext []byte) (*EncryptResult, error)` | `{Ciphertext: hex}` | encrypt for peer |
| `DecryptMessage(peerID, ciphertextHex string) (*DecryptResult, error)` | `{Plaintext: []byte}` | decrypt from peer |
| `HasSharedSecret(peerID string) bool` | — | check if shared secret exists |

**Types:**
- `EncryptResult` — `Ciphertext string`
- `DecryptResult` — `Plaintext []byte`

**State:** `MobileNode.e2eeKey *crypto.KeyPair` + `MobileNode.sharedSec map[string][]byte`

### E2EE Key Exchange Protocol

1. Each party calls `GenerateE2EEKey()` — stores local key pair
2. Party A sends public key via signaling: `SendSignal(sessionID, "e2ee_key", pubKeyHex)`
3. Party B receives signal, calls `SetRemoteKey(peerID_A, pubKeyHex_A)` — derives shared secret
4. Party B sends its pub key back: `SendSignal(sessionID, "e2ee_key", pubKeyHex_B)`
5. Party A calls `SetRemoteKey(peerID_B, pubKeyHex_B)` — derives same shared secret

**Client-side (PeerConnectionManager):** automatic key exchange on session establishment. Signal type `e2ee_key` is intercepted before SDP/ICE processing.

### Dart — `lib/services/e2ee_service.dart`

**Abstract class `E2eeService`:**

| Method | Returns | Description |
|---|---|---|
| `get isAvailable` | `bool` | E2EE ready for use |
| `get publicKey` | `String?` | local public key (base64 in mock, hex in prod) |
| `generateKey()` | `Future<String?>` | generate/reset key pair |
| `setRemoteKey(peerID, pubKeyHex)` | `Future<bool>` | store peer key, derive secret |
| `hasSharedSecret(peerID)` | `bool` | check if shared secret exists |
| `encryptMessage(peerID, plaintext)` | `Future<String?>` | encrypt, returns base64 |
| `decryptMessage(peerID, ciphertext)` | `Future<String?>` | decrypt, returns plaintext |

**Implementations:**
- `E2eeBridgeService` — calls Go bridge (production), currently `isAvailable = false` until bridge integration
- `E2eeMockService` — XOR-based mock for dev (uses `dart:convert` + `dart:math`), always `isAvailable = true`

### Integration in PeerConnectionManager

- `PeerConnectionManager` takes optional `E2eeService?` in constructor
- `start()` — auto-generates key if E2EE available
- `initiateCall()` / `handleIncomingSession()` — calls `_maybeExchangeE2eeKey(session)`
- `handleSignalMessage()` — intercepts `e2ee_key` type → `_handleE2eeKeyExchange()`
- `sendData()` — encrypts before sending if shared secret exists
- `onDataChannel` handler — decrypts incoming messages if shared secret exists
- `isE2eeActive` getter — true when key exchange completed
- `hasE2eeWith(peerID)` — check per-peer

#### `P2pBridgePlugin.kt` (Android)

- Implements `MethodCallHandler` + `EventChannel.StreamHandler`
- Uses reflection: `Class.forName("bridge.MobileNode")` — compiles without .aar
- `tryLoadBridge()` — callableName-based lookup for each method (Java method names are camelCase from Go)
- Event loop: creates coroutine that polls for events (kludge since gomobile doesn't support callbacks natively)
- Golang method naming convention: `getPeerID()` → `getPeerID(...)`, `startSignaling()` → `startSignaling(...)`

#### `P2pBridgePlugin.swift` (iOS)

- Implements `FlutterPlugin` + `FlutterStreamHandler`
- Direct import: `import p2p_node` (Swift module from .xcframework)
- `handle(_ call: FlutterMethodCall, result: @escaping FlutterResult)` — switch on `call.method`
- Calls `MobileNode` methods directly: `bridge.createNode(...)` etc.

---

## Multiaddr Protocol Codes

Used via `multiaddr.P_*` constants:

| Constant | Code | Used in |
|---|---|---|
| `P_TCP` | 6 | `ListenPort()` in node.go |
| `P_CIRCUIT` | 290 | `CircuitAddr()` in relay.go |
| `P_P2P` | 421 | `Multiaddrs()` in node.go |

---

## Test Coverage

### `node_test.go` (6 tests)

| Test | What it covers |
|---|---|
| `TestNewNode_Default` | `NewNode(WithPort(0))` — creates node, has peer ID |
| `TestNode_PeerID_Deterministic` | `WithPrivateKey` — same key = same ID |
| `TestNode_Multiaddrs` | `Multiaddrs()` — returns addr with peer ID suffix |
| `TestNode_Close_Twice` | `Close()` idempotent |
| `TestNode_WithListenAddrs` | `WithListenAddrs` — custom listen |
| `TestNode_ConnManager` | `WithConnectionManager` — connmgr enabled |

### `dht_test.go` (6 tests)

| Test | What it covers |
|---|---|
| `TestDHT_New` | `NewDHT(n, WithDHTMode(Server))` |
| `TestDHT_Bootstrap_NoPeers` | `Bootstrap` without peers = error |
| `TestDHT_FindPeer_Unknown` | `FindPeer` unknown ID = error |
| `TestDHT_TwoNodes_FindPeer` | Connect + WaitForPeer + FindPeer (2 nodes) |
| `TestDHT_Provide_Find` | Provide + FindProviders round-trip |
| `TestDHT_WithBootstrapPeers` | `WithBootstrapPeers` — configured in DHT |

**Helper:** `newTestNode(t)` creates node with cleanup.

### `mdns_test.go` (4 tests, 1 skip)

| Test | What it covers |
|---|---|
| `TestNewDiscovery_Valid` | `NewDiscovery` creates Discovery, Service not nil |
| `TestNewDiscovery_NilNotifee` | nil notifee is allowed |
| `TestDiscovery_StartClose` | Start + Close |
| `TestDiscovery_StartTwice` | Start called twice is safe |
| `TestDiscovery_Integration` | 2-node mDNS exchange (SKIP on non-Linux) |

**Test helpers:** `testNotifee` (implements Notifee with atomic count + `Count()` getter), `mdnsNotifeeFunc` (func wrapper for Notifee).

### `relay_test.go` (3 tests)

| Test | What it covers |
|---|---|
| `TestRelay_NewRelay` | `NewRelay(node)` creates relay |
| `TestRelay_ThreeNodes` | relay + 2 clients: connect → reserve → stream via circuit |
| `TestRelay_CircuitAddr` | `CircuitAddr` builds valid `/p2p-circuit/p2p/` addr |

**Protocol:** `/p2p/test/relay/1.0.0`

### `signaling_test.go` (3 tests)

| Test | What it covers |
|---|---|
| `TestSignaling_DialAndExchange` | A→B offer, B→A answer round-trip |
| `TestSignaling_Close` | Start → Close → Start works |
| `TestSignaling_DialBeforeHandler` | Dial with empty handler works |

### `crypto/e2ee_test.go` (11 tests)

| Test | What it covers |
|---|---|
| `TestGenerateKey` | `GenerateKey()` — valid key pair |
| `TestGenerateKey_Unique` | two calls return different keys |
| `TestSharedSecret_Same` | A-B and B-A produce the same secret |
| `TestEncryptDecrypt` | round-trip with valid shared secret |
| `TestEncryptDecrypt_WrongKey` | wrong key = `ErrDecrypt` |
| `TestEncryptDecrypt_Empty` | empty plaintext round-trip |
| `TestEncryptDecrypt_Large` | 10KB round-trip |
| `TestDecrypt_Tampered` | flipped byte = auth failure |
| `TestMarshal_Unmarshal` | hex serialization round-trip |
| `TestUnmarshal_InvalidHex` | invalid hex returns error |
| `TestUnmarshal_WrongLength` | wrong key size returns error |

**Note:** `Package crypto_test` in `crypto/` directory (same package, internal test).

### `bridge/tests/mobilenode_test.go` (16 tests)

| Test | What it covers |
|---|---|
| `TestNewMobileNode` | `NewMobileNode(handler)` creates with empty state |
| `TestCreateNode` | `CreateNode(0, "")` — returns valid peer ID JSON |
| `TestCreateNode_Deterministic` | same keyHex → same peer ID |
| `TestGetPeerID` | `GetPeerID()` matches `CreateNode` result |
| `TestGetMultiaddrs` | `GetMultiaddrs()` returns non-empty addrs |
| `TestSetupDHT` | `SetupDHT(1, "")` — DHT in server mode |
| `TestProvide_FindProviders` | `Provide(key)` + `FindProviders(key)` round-trip |
| `TestStartSignaling` | `StartSignaling()` with handler set |
| `TestDialSignaling` | A→B `DialSignaling`, B receives `incoming_session` |
| `TestSendSignal` | A→B offer/answer round-trip via signaling |
| `TestMobileNode_Close` | `Close()` — cleanup |
| `TestMobileNode_Close_Idempotent` | `Close()` twice = no error |
| `TestMobileNode_GenerateE2EEKey` | `GenerateE2EEKey()` returns 64-char hex |
| `TestMobileNode_SetRemoteKey` | both sides set keys = shared secret exists |
| `TestMobileNode_EncryptDecryptMessage` | A encrypts → B decrypts = original |
| `TestMobileNode_Encrypt_WrongPeer` | encrypt with unknown peer = error |
| `TestMobileNode_E2EE_SignalingExchange` | `e2ee_key` sent via signaling, received by peer |
| `TestMobileNode_E2EE_EncryptViaSignaling` | full flow: key exchange → encrypt → send → receive → decrypt |

**Test helpers:**
- `newTestHandler` — creates `*testHandler` implementing `EventHandler` with `events []string` + `mu sync.Mutex`
- `testHandler.eventCount()` — atomic count of received events
- `testHandler.waitForEvent(t, substring, timeout)` — polling wait for event matching substring

**Integration pattern:**
```go
handlerA := newTestHandler()
defer handlerA.Close()
nodeA := NewMobileNode(handlerA)
nodeA.CreateNode(0, "")

handlerB := newTestHandler()
defer handlerB.Close()
nodeB := NewMobileNode(handlerB)
nodeB.CreateNode(0, "")

// Connect A↔B
nodeA.Connect(nodeB.GetMultiaddrs())
handlerB.waitForEvent(t, "peer_found", 10*time.Second)

// Signaling
nodeA.DialSignaling(nodeB.GetPeerID())
...
```

**Note:** `WaitForPeer` used internally in `Provide_FindProviders` to poll DHT routing table.

---

## Common Testing Patterns

### p2p/tests/ — direct p2p tests

**Imports for all p2p test files:**
```go
import (
    "github.com/stretchr/testify/require"
    "github.com/whyskydie/p2p-node/p2p"
)
```

**Creating nodes in tests:**
```go
nodeA, err := p2p.NewNode(p2p.WithPort(0))
require.NoError(t, err)
defer nodeA.Close()
```

**Connecting nodes:**
```go
err = nodeA.Connect(nodeB.Multiaddrs()[0])
require.NoError(t, err)
```

**Context with timeout:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

**require vs assert:** use `require` for fatal assertions (stop test), `assert` for non-fatal.

### bridge/tests/ — gomobile bridge tests

**Imports for bridge test files:**
```go
import (
    "github.com/stretchr/testify/require"
    "github.com/whyskydie/p2p-node/bridge"
)
```

**Creating MobileNodes in tests:**
```go
handler := newTestHandler()
defer handler.Close()
node := bridge.NewMobileNode(handler)
node.CreateNode(0, "")
```

**Event-based integration (two nodes):**
```go
nodeA.Connect(nodeB.GetMultiaddrs())
handlerB.waitForEvent(t, "peer_found", 10*time.Second)
```

**waitForEvent pattern (bridge package, not exported):**
```go
func (h *testHandler) waitForEvent(t *testing.T, substr string, timeout time.Duration) {
    deadline := time.After(timeout)
    for {
        h.mu.Lock()
        for _, e := range h.events {
            if strings.Contains(e, substr) {
                h.mu.Unlock()
                return
            }
        }
        h.mu.Unlock()
        select {
        case <-deadline:
            t.Fatalf("timeout waiting for event containing %q", substr)
        case <-time.After(10 * time.Millisecond):
        }
    }
}
```

---

## Project Conventions

### Go

1. **TDD:** tests first, then implementation
2. **Functional options** (`Option`, `DHTOption`) for constructors
3. **Error wrapping:** `fmt.Errorf("context: %w", err)`
4. **Idempotent Close:** `sync.Once` for Close methods
5. **All tests in external package** `p2p_test` in `p2p/tests/` (and `bridge/tests/`)
6. **Exported fields** when tests need access (e.g. `Discovery.Service`, `Discovery.Notifee`)
7. **Mutex on Read/Write** in SignalSession for concurrent safety
8. **Separate `Close()`** method even for test teardowns (`t.Cleanup` or `defer`)
9. **Test naming:** `Test<Subject>_<Scenario>`
10. **No central server** — all signaling/relay over libp2p streams
11. **gomobile bridge**: all exported methods take/return `string` (JSON); `EventHandler` interface has single `HandleEvent(string)` method
12. **Reflection in Kotlin:** `Class.forName("bridge.MobileNode")` — Kotlin plugin compiles without .aar

### Flutter/Dart

13. **Abstract service interface** (`P2pBridgeService`) — switchable impl: `P2pBridgeChannel` (production) / `P2pBridgeMock` (dev)
14. **Platform Channels:** `MethodChannel("p2p_bridge")` + `EventChannel("p2p_events")` — method names match Go bridge method names
15. **WebRTC config:** pass `Map<String, dynamic>` instead of `RTCIceServer` (webrtc_interface 1.5.1 removed the type)
16. **JSON serialization:** all cross-boundary data uses JSON strings (`P2pEvent.fromMap`, `SignalMessage` in Go)
17. **No Android/iOS-specific logic in Dart** — all platform details in native plugins (Kotlin/Swift)
18. **E2EE in Go:** X25519 + ChaCha20-Poly1305 in `crypto/e2ee.go`; bridge exposes via `GenerateE2EEKey/SetRemoteKey/EncryptMessage/DecryptMessage`; Dart `E2eeService` abstracts it
19. **Key exchange over signaling:** use signal type `e2ee_key` with hex public key; `PeerConnectionManager` auto-intercepts on session start
20. **DataChannel + E2EE:** encrypt on send (`sendData`), decrypt on receive (`onDataChannel`); `E2eeMockService` uses XOR for dev (base64 encoding)
