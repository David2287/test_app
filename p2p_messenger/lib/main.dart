import 'package:flutter/material.dart';
import 'services/p2p_bridge_service.dart';
import 'services/p2p_bridge_mock.dart';
import 'services/e2ee_service.dart';
import 'webrtc/peer_connection_manager.dart';
import 'models/p2p_event.dart';

export 'services/p2p_bridge_service.dart';
export 'services/p2p_bridge_mock.dart';
export 'services/e2ee_service.dart';
export 'webrtc/peer_connection_manager.dart';
export 'models/p2p_event.dart';

void main() {
  runApp(const P2pApp());
}

class P2pApp extends StatelessWidget {
  const P2pApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'P2P Messenger',
      theme: ThemeData.dark(useMaterial3: true),
      home: const HomeScreen(),
    );
  }
}

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  late final P2pBridgeService _bridge;
  late final PeerConnectionManager _webrtc;
  late final E2eeService _e2ee;
  final _peerIdController = TextEditingController();
  final _messageController = TextEditingController();
  final _log = <String>[];
  String? _myPeerId;
  bool _initialized = false;
  String? _lastSessionId;

  @override
  void initState() {
    super.initState();
    _e2ee = E2eeMockService();
    _bridge = P2pBridgeMock();
    _webrtc = PeerConnectionManager(_bridge, e2ee: _e2ee);
    _initBridge();
  }

  Future<void> _initBridge() async {
    try {
      await _bridge.createNode();
      _webrtc.start();
      _webrtc.onDataMessage = (sessionId, message) {
        _addLog('Data($sessionId): $message');
      };
      _bridge.events.listen(_onEvent);
      setState(() {
        _myPeerId = _bridge.peerId;
        _initialized = true;
      });
      _addLog('Node: $_myPeerId');
    } catch (e) {
      _addLog('Error: $e');
    }
  }

  void _onEvent(P2pEvent event) {
    if (event.isPeerDiscovered) {
      _addLog('Discovered: ${event.peerId}');
    } else if (event.isSignalMessage && event.signalType == 'e2ee_key') {
      _addLog('E2EE key received from ${event.peerId}');
    } else if (event.isSignalMessage && event.signalType == 'offer') {
      _addLog('Incoming call from ${event.peerId}');
    }
  }

  void _addLog(String msg) {
    final now = DateTime.now().toString().substring(11, 19);
    setState(() => _log.add('[$now] $msg'));
  }

  Future<void> _connect() async {
    final addr = _peerIdController.text.trim();
    if (addr.isEmpty) return;
    try {
      await _bridge.connect(addr);
      _addLog('Connected: $addr');
    } catch (e) {
      _addLog('Connect error: $e');
    }
  }

  Future<void> _call() async {
    final pid = _peerIdController.text.trim();
    if (pid.isEmpty) return;
    try {
      final sessionId = await _webrtc.initiateCall(pid);
      _lastSessionId = sessionId;
      _addLog('Calling $pid (session: $sessionId)');
    } catch (e) {
      _addLog('Call error: $e');
    }
  }

  Future<void> _send() async {
    final msg = _messageController.text.trim();
    if (msg.isEmpty || _lastSessionId == null) return;
    try {
      await _webrtc.sendData(_lastSessionId!, msg);
      _addLog('Sent: $msg');
      _messageController.clear();
    } catch (e) {
      _addLog('Send error: $e');
    }
  }

  @override
  void dispose() {
    _webrtc.dispose();
    _bridge.close();
    _peerIdController.dispose();
    _messageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final e2eeActive = _webrtc.isE2eeActive;
    return Scaffold(
      appBar: AppBar(
        title: Text('P2P: ${_myPeerId?.substring(0, 16) ?? "init..."}'),
        actions: [
          if (_initialized)
            Padding(
              padding: const EdgeInsets.only(right: 12),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(e2eeActive ? 'E2EE' : 'no E2EE',
                      style: TextStyle(
                          fontSize: 11,
                          color: e2eeActive ? Colors.green : Colors.grey)),
                  const SizedBox(width: 4),
                  Icon(
                    e2eeActive ? Icons.lock : Icons.lock_open,
                    size: 16,
                    color: e2eeActive ? Colors.green : Colors.grey,
                  ),
                ],
              ),
            ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(8),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _peerIdController,
                    decoration: const InputDecoration(
                      hintText: 'Peer ID or multiaddr',
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                FilledButton.tonal(
                  onPressed: _initialized ? _connect : null,
                  child: const Text('Connect'),
                ),
                const SizedBox(width: 4),
                FilledButton(
                  onPressed: _initialized ? _call : null,
                  child: const Text('Call'),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _messageController,
                    decoration: const InputDecoration(
                      hintText: 'Message',
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                    onSubmitted: (_) => _send(),
                  ),
                ),
                const SizedBox(width: 8),
                FilledButton.tonalIcon(
                  onPressed: _initialized && _lastSessionId != null ? _send : null,
                  icon: const Icon(Icons.send, size: 16),
                  label: const Text('Send'),
                ),
              ],
            ),
          ),
          const SizedBox(height: 4),
          Expanded(
            child: ListView.builder(
              reverse: true,
              itemCount: _log.length,
              itemBuilder: (_, i) => Padding(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 1),
                child: Text(
                  _log[_log.length - 1 - i],
                  style: const TextStyle(fontSize: 11, fontFamily: 'monospace'),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
