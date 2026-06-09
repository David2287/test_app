import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import '../models/p2p_event.dart';
import '../services/p2p_bridge_service.dart';
import '../services/e2ee_service.dart';

class PeerConnectionManager {
  final P2pBridgeService _bridge;
  final E2eeService? _e2ee;
  final Map<String, SessionContext> _sessions = {};
  StreamSubscription<P2pEvent>? _eventSub;
  bool _e2eeExchanged = false;

  PeerConnectionManager(this._bridge, {E2eeService? e2ee}) : _e2ee = e2ee;

  StreamSubscription<P2pEvent>? get eventSubscription => _eventSub;

  static Map<String, dynamic> get _config => {
    'iceServers': [
      {'urls': 'stun:stun.l.google.com:19302'},
    ],
  };

  Future<void> start() async {
    _eventSub = _bridge.events.listen(_onEvent);

    final e2ee = _e2ee;
    if (e2ee != null && e2ee.isAvailable) {
      final pk = e2ee.publicKey;
      if (pk == null) {
        await e2ee.generateKey();
      }
    }
  }

  Future<String> initiateCall(String peerId) async {
    final sessionId = await _bridge.dialSignal(peerId);

    final pc = await createPeerConnection(_config);

    final session = SessionContext(sessionId: sessionId, peerId: peerId, pc: pc);
    _sessions[sessionId] = session;

    _setupPcListeners(session);

    _maybeExchangeE2eeKey(session);

    final offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    await _bridge.sendSignal(sessionId, 'offer', jsonEncode({
      'sdp': offer.sdp,
      'type': offer.type,
    }));

    return sessionId;
  }

  void _maybeExchangeE2eeKey(SessionContext session) async {
    final e2ee = _e2ee;
    if (e2ee == null || !e2ee.isAvailable) return;
    if (e2ee.hasSharedSecret(session.peerId)) return;
    final pk = e2ee.publicKey;
    if (pk == null) return;
    await _bridge.sendSignal(session.sessionId, 'e2ee_key', pk);
  }

  Future<void> handleIncomingSession(P2pEvent event) async {
    final sessionId = event.sessionId!;
    final peerId = event.peerId!;

    final pc = await createPeerConnection(_config);

    final session = SessionContext(sessionId: sessionId, peerId: peerId, pc: pc);
    _sessions[sessionId] = session;

    _setupPcListeners(session);

    _maybeExchangeE2eeKey(session);
  }

  Future<void> handleSignalMessage(P2pEvent event) async {
    final sessionId = event.sessionId!;
    final type = event.signalType!;
    final data = event.signalData!;

    if (type == 'e2ee_key') {
      await _handleE2eeKeyExchange(sessionId, data);
      return;
    }

    final session = _sessions[sessionId];
    if (session == null) {
      debugPrint('[WebRTC] no session $sessionId for signal $type');
      return;
    }

    try {
      final parsed = jsonDecode(data) as Map<String, dynamic>;
      final sdp = parsed['sdp'] as String?;
      final candidate = parsed['candidate'] as String?;
      final sdpMid = parsed['sdpMid'] as String?;
      final sdpMLineIndex = parsed['sdpMLineIndex'] as int?;

      if (type == 'offer') {
        await session.pc.setRemoteDescription(
          RTCSessionDescription(sdp ?? '', 'offer'),
        );
        final answer = await session.pc.createAnswer();
        await session.pc.setLocalDescription(answer);
        await _bridge.sendSignal(sessionId, 'answer', jsonEncode({
          'sdp': answer.sdp,
          'type': answer.type,
        }));
      } else if (type == 'answer') {
        await session.pc.setRemoteDescription(
          RTCSessionDescription(sdp ?? '', 'answer'),
        );
      } else if (type == 'ice-candidate' && candidate != null) {
        await session.pc.addCandidate(
          RTCIceCandidate(candidate, sdpMid ?? '', sdpMLineIndex ?? 0),
        );
      }
    } catch (e) {
      debugPrint('[WebRTC] handleSignal error: $e');
    }
  }

  Future<void> _handleE2eeKeyExchange(String sessionId, String pubKeyHex) async {
    final e2ee = _e2ee;
    if (e2ee == null || !e2ee.isAvailable) return;
    final session = _sessions[sessionId];
    if (session == null) return;

    final ok = await e2ee.setRemoteKey(session.peerId, pubKeyHex);
    if (ok && !_e2eeExchanged) {
      _e2eeExchanged = true;
      final pk = e2ee.publicKey;
      if (pk != null) {
        await _bridge.sendSignal(sessionId, 'e2ee_key', pk);
      }
      debugPrint('[E2EE] key exchange complete with ${session.peerId}');
    }
  }

  void _setupPcListeners(SessionContext session) {
    session.pc.onIceCandidate = (RTCIceCandidate candidate) {
      _bridge.sendSignal(session.sessionId, 'ice-candidate', jsonEncode({
        'candidate': candidate.candidate,
        'sdpMid': candidate.sdpMid,
        'sdpMLineIndex': candidate.sdpMLineIndex,
      }));
    };

    session.pc.onIceConnectionState = (state) {
      debugPrint('[WebRTC] $session: ICE $state');
      if (state == RTCIceConnectionState.RTCIceConnectionStateDisconnected ||
          state == RTCIceConnectionState.RTCIceConnectionStateFailed ||
          state == RTCIceConnectionState.RTCIceConnectionStateClosed) {
        _sessions.remove(session.sessionId);
      }
    };

    session.pc.onDataChannel = (channel) {
      debugPrint('[WebRTC] $session: data channel ${channel.label}');
      session.dataChannel = channel;
      final e2ee = _e2ee;
      channel.onMessage = (message) {
        final text = message.text;
        if (e2ee != null && e2ee.hasSharedSecret(session.peerId)) {
          e2ee.decryptMessage(session.peerId, text).then((decrypted) {
            if (decrypted != null) {
              _onDataMessage?.call(session.sessionId, decrypted);
            }
          });
        } else {
          _onDataMessage?.call(session.sessionId, text);
        }
      };
    };
  }

  void _onEvent(P2pEvent event) {
    if (event.isSignalSession) {
      handleIncomingSession(event);
    } else if (event.isSignalMessage) {
      handleSignalMessage(event);
    } else if (event.isSignalSessionClosed) {
      _sessions.remove(event.sessionId);
      debugPrint('[WebRTC] session closed: ${event.sessionId}');
    }
  }

  void Function(String sessionId, String message)? _onDataMessage;

  set onDataMessage(void Function(String sessionId, String message)? cb) {
    _onDataMessage = cb;
  }

  Future<void> sendData(String sessionId, String message) async {
    final session = _sessions[sessionId];
    final dc = session?.dataChannel;
    if (dc == null) return;
    final e2ee = _e2ee;
    if (e2ee != null && e2ee.hasSharedSecret(session!.peerId)) {
      final encrypted = await e2ee.encryptMessage(session.peerId, message);
      if (encrypted != null) {
        dc.send(RTCDataChannelMessage(encrypted));
        return;
      }
    }
    dc.send(RTCDataChannelMessage(message));
  }

  bool get isE2eeActive {
    final e2ee = _e2ee;
    return e2ee != null && e2ee.isAvailable && _e2eeExchanged;
  }

  bool hasE2eeWith(String peerId) {
    return _e2ee?.hasSharedSecret(peerId) ?? false;
  }

  RTCPeerConnection? getConnection(String sessionId) {
    return _sessions[sessionId]?.pc;
  }

  Future<void> closeSession(String sessionId) async {
    final session = _sessions.remove(sessionId);
    await session?.pc.close();
    await _bridge.closeSignalSession(sessionId);
  }

  Future<void> dispose() async {
    await _eventSub?.cancel();
    for (final session in _sessions.values) {
      await session.pc.close();
    }
    _sessions.clear();
  }
}

class SessionContext {
  final String sessionId;
  final String peerId;
  final RTCPeerConnection pc;
  RTCDataChannel? dataChannel;

  SessionContext({
    required this.sessionId,
    required this.peerId,
    required this.pc,
  });

  @override
  String toString() => 'Session($sessionId, $peerId)';
}
