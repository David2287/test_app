import Flutter
import UIKit

public class P2pBridgePlugin: NSObject, FlutterPlugin, FlutterStreamHandler {
    private var methodChannel: FlutterMethodChannel?
    private var eventChannel: FlutterEventChannel?
    private var eventSink: FlutterEventSink?
    
    private var mobileNode: MobileNode?
    private var goHandler: GoEventHandler?
    
    public static func register(with registrar: FlutterPluginRegistrar) {
        let instance = P2pBridgePlugin()
        
        instance.methodChannel = FlutterMethodChannel(
            name: "p2p_bridge",
            binaryMessenger: registrar.messenger()
        )
        instance.methodChannel?.setMethodCallHandler(instance.handle)
        
        instance.eventChannel = FlutterEventChannel(
            name: "p2p_events",
            binaryMessenger: registrar.messenger()
        )
        instance.eventChannel?.setStreamHandler(instance)
    }
    
    public func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
        eventSink = events
        return nil
    }
    
    public func onCancel(withArguments arguments: Any?) -> FlutterError? {
        eventSink = nil
        return nil
    }
    
    public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
        do {
            try handleMethod(call, result: result)
        } catch {
            result(FlutterError(code: "BRIDGE_ERROR", message: error.localizedDescription, details: nil))
        }
    }
    
    private func handleMethod(_ call: FlutterMethodCall, result: @escaping FlutterResult) throws {
        switch call.method {
        case "createNode":
            let handler = GoEventHandler(eventSink: eventSink)
            goHandler = handler
            mobileNode = NewMobileNode(handler)
            result(mobileNode?.peerID())
            
        case "peerId":
            guard let node = mobileNode else {
                throw BridgeError.nodeNotCreated
            }
            result(node.peerID())
            
        case "addresses":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            result(node.addressesString())
            
        case "connect":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let addr = getArg(call, "addr") else { throw BridgeError.missingArg("addr") }
            try node.connect(addr)
            result(nil)
            
        case "disconnect":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let peerId = getArg(call, "peerId") else { throw BridgeError.missingArg("peerId") }
            try node.disconnect(peerId)
            result(nil)
            
        case "startDht":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            let peers = getArg(call, "bootstrapPeers") as? [String] ?? []
            try node.startDHT(peers)
            result(nil)
            
        case "findPeer":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let peerId = getArg(call, "peerId") else { throw BridgeError.missingArg("peerId") }
            let addrs = try node.findPeer(peerId)
            result(addrs)
            
        case "provide":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let key = getArg(call, "key") else { throw BridgeError.missingArg("key") }
            try node.provide(key)
            result(nil)
            
        case "findProviders":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let key = getArg(call, "key") else { throw BridgeError.missingArg("key") }
            let providers = try node.findProviders(key)
            result(providers)
            
        case "startDiscovery":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let name = getArg(call, "serviceName") else { throw BridgeError.missingArg("serviceName") }
            try node.startDiscovery(name)
            result(nil)
            
        case "stopDiscovery":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            try node.stopDiscovery()
            result(nil)
            
        case "startRelay":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            try node.startRelay()
            result(nil)
            
        case "reserveRelay":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let addr = getArg(call, "relayAddr") else { throw BridgeError.missingArg("relayAddr") }
            let circuitAddr = try node.reserveRelay(addr)
            result(circuitAddr)
            
        case "startSignaling":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            try node.startSignaling()
            result(nil)
            
        case "dialSignal":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let peerId = getArg(call, "peerId") else { throw BridgeError.missingArg("peerId") }
            let sessionId = try node.dialSignal(peerId)
            result(sessionId)
            
        case "sendSignal":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let sid = getArg(call, "sessionId") else { throw BridgeError.missingArg("sessionId") }
            guard let type = getArg(call, "type") else { throw BridgeError.missingArg("type") }
            guard let data = getArg(call, "data") else { throw BridgeError.missingArg("data") }
            try node.sendSignal(sid, type, data)
            result(nil)
            
        case "closeSignalSession":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let sid = getArg(call, "sessionId") else { throw BridgeError.missingArg("sessionId") }
            try node.closeSignalSession(sid)
            result(nil)
            
        case "waitForPeer":
            guard let node = mobileNode else { throw BridgeError.nodeNotCreated }
            guard let pid = getArg(call, "peerId") else { throw BridgeError.missingArg("peerId") }
            try node.waitForPeer(pid)
            result(nil)
            
        case "close":
            mobileNode?.close()
            mobileNode = nil
            goHandler = nil
            result(nil)
            
        default:
            result(FlutterMethodNotImplemented)
        }
    }
    
    private func getArg(_ call: FlutterMethodCall, _ key: String) -> String? {
        return (call.arguments as? [String: Any])?[key] as? String
    }
}

private class GoEventHandler {
    private let eventSink: FlutterEventSink?
    
    init(eventSink: FlutterEventSink?) {
        self.eventSink = eventSink
    }
    
    func onPeerDiscovered(_ peerId: String, addrs: String) {
        emit("peer_discovered", ["peerId": peerId, "addrs": addrs])
    }
    
    func onSignalSession(_ sessionId: String, peerId: String) {
        emit("signal_session", ["sessionId": sessionId, "peerId": peerId])
    }
    
    func onSignalMessage(_ sessionId: String, type: String, data: String) {
        emit("signal_message", ["sessionId": sessionId, "type": type, "data": data])
    }
    
    func onSignalSessionClosed(_ sessionId: String) {
        emit("signal_closed", ["sessionId": sessionId])
    }
    
    func onError(_ message: String) {
        emit("error", ["message": message])
    }
    
    private func emit(_ event: String, _ data: [String: String]) {
        var payload: [String: Any] = ["event": event]
        payload.merge(data) { $1 }
        DispatchQueue.main.async { [weak self] in
            self?.eventSink?(payload)
        }
    }
}

enum BridgeError: Error, LocalizedError {
    case nodeNotCreated
    case missingArg(String)
    
    var errorDescription: String? {
        switch self {
        case .nodeNotCreated: return "call createNode first"
        case .missingArg(let key): return "missing argument: \(key)"
        }
    }
}
