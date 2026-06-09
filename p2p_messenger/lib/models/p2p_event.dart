class P2pEvent {
  final String type;
  final Map<String, dynamic> data;

  P2pEvent({required this.type, required this.data});

  factory P2pEvent.fromMap(Map<dynamic, dynamic> map) {
    return P2pEvent(
      type: map['event'] as String? ?? 'unknown',
      data: Map<String, dynamic>.from(map),
    );
  }

  bool get isPeerDiscovered => type == 'peer_discovered';
  bool get isSignalSession => type == 'signal_session';
  bool get isSignalMessage => type == 'signal_message';
  bool get isSignalSessionClosed => type == 'signal_closed';
  bool get isError => type == 'error';

  String? get peerId => data['peerId'] as String?;
  String? get sessionId => data['sessionId'] as String?;
  String? get signalType => data['type'] as String?;
  String? get signalData => data['data'] as String?;
  String? get message => data['message'] as String?;
  String? get addrs => data['addrs'] as String?;
}
