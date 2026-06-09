import 'dart:async';
import 'package:flutter/services.dart';
import '../models/p2p_event.dart';

abstract class P2pBridgeService {
  Stream<P2pEvent> get events;

  Future<String> createNode();
  String get peerId;
  Future<List<String>> addresses();
  Future<void> connect(String addr);
  Future<void> disconnect(String peerId);
  Future<void> startDht(List<String>? bootstrapPeers);
  Future<List<String>> findPeer(String peerId);
  Future<void> provide(String key);
  Future<List<String>> findProviders(String key);
  Future<void> startDiscovery(String serviceName);
  Future<void> stopDiscovery();
  Future<void> startRelay();
  Future<String> reserveRelay(String relayAddr);
  Future<void> startSignaling();
  Future<String> dialSignal(String peerId);
  Future<void> sendSignal(String sessionId, String type, String data);
  Future<void> closeSignalSession(String sessionId);
  Future<void> waitForPeer(String peerId);
  Future<void> close();
}

class P2pBridgeChannel implements P2pBridgeService {
  static const _methodChannel = MethodChannel('p2p_bridge');
  static const _eventChannel = EventChannel('p2p_events');

  String? _peerId;
  StreamSubscription<dynamic>? _eventSubscription;
  StreamController<P2pEvent>? _controller;

  @override
  Stream<P2pEvent> get events {
    _controller ??= StreamController<P2pEvent>.broadcast();
    _eventSubscription ??= _eventChannel
        .receiveBroadcastStream()
        .map((e) => P2pEvent.fromMap(e as Map<dynamic, dynamic>))
        .listen(
          (event) => _controller?.add(event),
          onError: (e) => _controller?.addError(e),
        );
    return _controller!.stream;
  }

  @override
  Future<String> createNode() async {
    await _methodChannel.invokeMethod('createNode');
    _peerId = await _methodChannel.invokeMethod('peerId');
    return _peerId!;
  }

  @override
  String get peerId => _peerId ?? '';

  @override
  Future<List<String>> addresses() async {
    final result = await _methodChannel.invokeMethod('addresses');
    return List<String>.from(result as List);
  }

  @override
  Future<void> connect(String addr) async {
    await _methodChannel.invokeMethod('connect', {'addr': addr});
  }

  @override
  Future<void> disconnect(String peerId) async {
    await _methodChannel.invokeMethod('disconnect', {'peerId': peerId});
  }

  @override
  Future<void> startDht(List<String>? bootstrapPeers) async {
    await _methodChannel.invokeMethod('startDht', {
      'bootstrapPeers': bootstrapPeers ?? [],
    });
  }

  @override
  Future<List<String>> findPeer(String peerId) async {
    final result = await _methodChannel.invokeMethod('findPeer', {'peerId': peerId});
    return List<String>.from(result as List);
  }

  @override
  Future<void> provide(String key) async {
    await _methodChannel.invokeMethod('provide', {'key': key});
  }

  @override
  Future<List<String>> findProviders(String key) async {
    final result = await _methodChannel.invokeMethod('findProviders', {'key': key});
    return List<String>.from(result as List);
  }

  @override
  Future<void> startDiscovery(String serviceName) async {
    await _methodChannel.invokeMethod('startDiscovery', {'serviceName': serviceName});
  }

  @override
  Future<void> stopDiscovery() async {
    await _methodChannel.invokeMethod('stopDiscovery');
  }

  @override
  Future<void> startRelay() async {
    await _methodChannel.invokeMethod('startRelay');
  }

  @override
  Future<String> reserveRelay(String relayAddr) async {
    final result = await _methodChannel.invokeMethod('reserveRelay', {'relayAddr': relayAddr});
    return result as String;
  }

  @override
  Future<void> startSignaling() async {
    await _methodChannel.invokeMethod('startSignaling');
  }

  @override
  Future<String> dialSignal(String peerId) async {
    final result = await _methodChannel.invokeMethod('dialSignal', {'peerId': peerId});
    return result as String;
  }

  @override
  Future<void> sendSignal(String sessionId, String type, String data) async {
    await _methodChannel.invokeMethod('sendSignal', {
      'sessionId': sessionId,
      'type': type,
      'data': data,
    });
  }

  @override
  Future<void> closeSignalSession(String sessionId) async {
    await _methodChannel.invokeMethod('closeSignalSession', {'sessionId': sessionId});
  }

  @override
  Future<void> waitForPeer(String peerId) async {
    await _methodChannel.invokeMethod('waitForPeer', {'peerId': peerId});
  }

  @override
  Future<void> close() async {
    await _eventSubscription?.cancel();
    _eventSubscription = null;
    await _controller?.close();
    _controller = null;
    await _methodChannel.invokeMethod('close');
  }
}
