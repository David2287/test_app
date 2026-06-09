import 'dart:async';
import 'package:flutter/foundation.dart';
import 'p2p_bridge_service.dart';
import '../models/p2p_event.dart';

class P2pBridgeMock implements P2pBridgeService {
  final _controller = StreamController<P2pEvent>.broadcast();
  String _peerId = '';

  @override
  Stream<P2pEvent> get events => _controller.stream;

  @override
  Future<String> createNode() async {
    _peerId = '12D3KooWMock${DateTime.now().millisecondsSinceEpoch}';
    debugPrint('[MockBridge] createNode -> $_peerId');
    return _peerId;
  }

  @override
  String get peerId => _peerId;

  @override
  Future<List<String>> addresses() async {
    return ['/ip4/127.0.0.1/tcp/0/p2p/$_peerId'];
  }

  @override
  Future<void> connect(String addr) async {
    debugPrint('[MockBridge] connect $addr');
  }

  @override
  Future<void> disconnect(String peerId) async {
    debugPrint('[MockBridge] disconnect $peerId');
  }

  @override
  Future<void> startDht(List<String>? bootstrapPeers) async {
    debugPrint('[MockBridge] startDht');
  }

  @override
  Future<List<String>> findPeer(String peerId) async {
    return ['/ip4/127.0.0.1/tcp/9999/p2p/$peerId'];
  }

  @override
  Future<void> provide(String key) async {
    debugPrint('[MockBridge] provide $key');
  }

  @override
  Future<List<String>> findProviders(String key) async {
    return ['12D3KooWMockProvider'];
  }

  @override
  Future<void> startDiscovery(String serviceName) async {
    debugPrint('[MockBridge] startDiscovery $serviceName');
  }

  @override
  Future<void> stopDiscovery() async {
    debugPrint('[MockBridge] stopDiscovery');
  }

  @override
  Future<void> startRelay() async {
    debugPrint('[MockBridge] startRelay');
  }

  @override
  Future<String> reserveRelay(String relayAddr) async {
    return '$relayAddr/p2p-circuit/p2p/$_peerId';
  }

  @override
  Future<void> startSignaling() async {
    debugPrint('[MockBridge] startSignaling');
  }

  @override
  Future<String> dialSignal(String peerId) async {
    final sessionId = 'session-${DateTime.now().millisecondsSinceEpoch}';
    debugPrint('[MockBridge] dialSignal -> $sessionId');
    return sessionId;
  }

  @override
  Future<void> sendSignal(String sessionId, String type, String data) async {
    debugPrint('[MockBridge] sendSignal $sessionId $type');
    _emitEvent(P2pEvent(
      type: 'signal_message',
      data: {
        'event': 'signal_message',
        'sessionId': sessionId,
        'type': type == 'offer' ? 'answer' : 'ice-candidate',
        'data': '{"sdp":"mock-sdp"}',
      },
    ));
  }

  @override
  Future<void> closeSignalSession(String sessionId) async {
    debugPrint('[MockBridge] closeSignalSession $sessionId');
  }

  @override
  Future<void> waitForPeer(String peerId) async {
    debugPrint('[MockBridge] waitForPeer $peerId');
  }

  @override
  Future<void> close() async {
    await _controller.close();
  }

  void _emitEvent(P2pEvent event) {
    if (!_controller.isClosed) {
      _controller.add(event);
    }
  }

  void emitEvent(P2pEvent event) => _emitEvent(event);
}
