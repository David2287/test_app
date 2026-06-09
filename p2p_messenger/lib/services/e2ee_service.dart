import 'dart:convert';
import 'dart:math';

abstract class E2eeService {
  bool get isAvailable;
  String? get publicKey;
  Future<String?> generateKey();
  Future<bool> setRemoteKey(String peerId, String pubKeyHex);
  bool hasSharedSecret(String peerId);
  Future<String?> encryptMessage(String peerId, String plaintext);
  Future<String?> decryptMessage(String peerId, String ciphertext);
}

class E2eeBridgeService implements E2eeService {
  @override
  bool get isAvailable => false; // Will be true when bridge is connected
  @override
  String? get publicKey => null;

  @override
  Future<String?> generateKey() async => null;

  @override
  Future<bool> setRemoteKey(String peerId, String pubKeyHex) async => false;

  @override
  bool hasSharedSecret(String peerId) => false;

  @override
  Future<String?> encryptMessage(String peerId, String plaintext) async => null;

  @override
  Future<String?> decryptMessage(String peerId, String ciphertext) async => null;
}

class E2eeMockService implements E2eeService {
  KeyPair? _keyPair;
  final _sharedSecrets = <String, String>{};
  final _remoteKeys = <String, String>{};
  static const _isAvailable = true;

  @override
  bool get isAvailable => _isAvailable;
  @override
  String? get publicKey => _keyPair?.publicKey;

  @override
  Future<String?> generateKey() async {
    _keyPair = KeyPair.generate();
    return _keyPair!.publicKey;
  }

  @override
  Future<bool> setRemoteKey(String peerId, String pubKeyHex) async {
    if (_keyPair == null) return false;
    _remoteKeys[peerId] = pubKeyHex;
    _sharedSecrets[peerId] = _keyPair!.deriveShared(pubKeyHex);
    return true;
  }

  @override
  bool hasSharedSecret(String peerId) => _sharedSecrets.containsKey(peerId);

  @override
  Future<String?> encryptMessage(String peerId, String plaintext) async {
    final secret = _sharedSecrets[peerId];
    if (secret == null) return null;
    return _xorEncrypt(plaintext, secret);
  }

  @override
  Future<String?> decryptMessage(String peerId, String ciphertext) async {
    final secret = _sharedSecrets[peerId];
    if (secret == null) return null;
    return _xorEncrypt(ciphertext, secret);
  }

  String _xorEncrypt(String input, String key) {
    final inputBytes = utf8.encode(input);
    final keyBytes = utf8.encode(key);
    final result = List<int>.generate(inputBytes.length,
        (i) => inputBytes[i] ^ keyBytes[i % keyBytes.length]);
    return base64Encode(result);
  }
}

class KeyPair {
  final String privateKey;
  final String publicKey;

  KeyPair({required this.privateKey, required this.publicKey});

  factory KeyPair.generate() {
    final random = Random.secure();
    final privBytes = List<int>.generate(32, (_) => random.nextInt(256));
    final pubBytes = List<int>.generate(32, (_) => random.nextInt(256));
    return KeyPair(
      privateKey: base64Encode(privBytes),
      publicKey: base64Encode(pubBytes),
    );
  }

  String deriveShared(String remotePubKeyHex) {
    final remoteBytes = base64Decode(remotePubKeyHex);
    final privBytes = base64Decode(privateKey);
    final shared = List<int>.generate(32,
        (i) => privBytes[i] ^ remoteBytes[i % remoteBytes.length]);
    return base64Encode(shared);
  }
}
