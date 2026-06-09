package com.whyskydie.p2p_messenger

import android.content.Context
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.EventChannel
import kotlinx.coroutines.*
import org.json.JSONObject

class P2pBridgePlugin : FlutterPlugin, MethodChannel.MethodCallHandler, EventChannel.StreamHandler {
    private lateinit var context: Context
    private lateinit var methodChannel: MethodChannel
    private lateinit var eventChannel: EventChannel
    private var eventSink: EventChannel.EventSink? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    private var mobileNode: Any? = null
    private var handler: GoEventHandler? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        context = binding.applicationContext
        methodChannel = MethodChannel(binding.binaryMessenger, "p2p_bridge")
        methodChannel.setMethodCallHandler(this)
        eventChannel = EventChannel(binding.binaryMessenger, "p2p_events")
        eventChannel.setStreamHandler(this)
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        methodChannel.setMethodCallHandler(null)
        eventChannel.setStreamHandler(null)
        scope.cancel()
    }

    override fun onListen(arguments: Any?, sink: EventChannel.EventSink) {
        eventSink = sink
    }

    override fun onCancel(arguments: Any?) {
        eventSink = null
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        scope.launch {
            try {
                handleMethod(call, result)
            } catch (e: Exception) {
                result.error("BRIDGE_ERROR", e.message ?: "unknown error", null)
            }
        }
    }

    private suspend fun handleMethod(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "createNode" -> {
                handler = GoEventHandler(eventSink, scope)
                // bridge.NewMobileNode(handler) -> stored in mobileNode
                // bridge.MobileNode.peerID() -> returned
                val bridgeClass = tryLoadBridge()
                if (bridgeClass != null) {
                    val node = bridgeClass.getMethod("NewMobileNode", Any::class.java)
                        .invoke(null, handler)
                    mobileNode = node
                    val peerId = node.javaClass.getMethod("peerID").invoke(node) as String
                    result.success(peerId)
                } else {
                    result.error("BRIDGE_UNAVAILABLE", "Go bridge library not loaded", null)
                }
            }
            "peerId" -> {
                if (mobileNode != null) {
                    val peerId = mobileNode!!.javaClass.getMethod("peerID").invoke(mobileNode) as String
                    result.success(peerId)
                } else {
                    result.error("NODE_NOT_CREATED", "Call createNode first", null)
                }
            }
            "addresses" -> {
                checkNode()
                val addrs = mobileNode!!.javaClass.getMethod("addressesString").invoke(mobileNode) as List<*>
                result.success(addrs)
            }
            "connect" -> {
                checkNode()
                val addr = call.argument<String>("addr")!!
                mobileNode!!.javaClass.getMethod("connect", String::class.java).invoke(mobileNode, addr)
                result.success(null)
            }
            "disconnect" -> {
                checkNode()
                val peerId = call.argument<String>("peerId")!!
                mobileNode!!.javaClass.getMethod("disconnect", String::class.java).invoke(mobileNode, peerId)
                result.success(null)
            }
            "startDht" -> {
                checkNode()
                @Suppress("UNCHECKED_CAST")
                val peers = call.argument<List<String>>("bootstrapPeers") ?: emptyList()
                mobileNode!!.javaClass.getMethod("startDHT", List::class.java)
                    .invoke(mobileNode, peers)
                result.success(null)
            }
            "findPeer" -> {
                checkNode()
                val peerId = call.argument<String>("peerId")!!
                val addrs = mobileNode!!.javaClass.getMethod("findPeer", String::class.java)
                    .invoke(mobileNode, peerId) as List<*>
                result.success(addrs)
            }
            "provide" -> {
                checkNode()
                val key = call.argument<String>("key")!!
                mobileNode!!.javaClass.getMethod("provide", String::class.java)
                    .invoke(mobileNode, key)
                result.success(null)
            }
            "findProviders" -> {
                checkNode()
                val key = call.argument<String>("key")!!
                val providers = mobileNode!!.javaClass.getMethod("findProviders", String::class.java)
                    .invoke(mobileNode, key) as List<*>
                result.success(providers)
            }
            "startDiscovery" -> {
                checkNode()
                val serviceName = call.argument<String>("serviceName")!!
                mobileNode!!.javaClass.getMethod("startDiscovery", String::class.java)
                    .invoke(mobileNode, serviceName)
                result.success(null)
            }
            "stopDiscovery" -> {
                checkNode()
                mobileNode!!.javaClass.getMethod("stopDiscovery").invoke(mobileNode)
                result.success(null)
            }
            "startRelay" -> {
                checkNode()
                mobileNode!!.javaClass.getMethod("startRelay").invoke(mobileNode)
                result.success(null)
            }
            "reserveRelay" -> {
                checkNode()
                val relayAddr = call.argument<String>("relayAddr")!!
                val circuitAddr = mobileNode!!.javaClass.getMethod("reserveRelay", String::class.java)
                    .invoke(mobileNode, relayAddr) as String
                result.success(circuitAddr)
            }
            "startSignaling" -> {
                checkNode()
                mobileNode!!.javaClass.getMethod("startSignaling").invoke(mobileNode)
                result.success(null)
            }
            "dialSignal" -> {
                checkNode()
                val peerId = call.argument<String>("peerId")!!
                val sessionId = mobileNode!!.javaClass.getMethod("dialSignal", String::class.java)
                    .invoke(mobileNode, peerId) as String
                result.success(sessionId)
            }
            "sendSignal" -> {
                checkNode()
                val sessionId = call.argument<String>("sessionId")!!
                val type = call.argument<String>("type")!!
                val data = call.argument<String>("data")!!
                mobileNode!!.javaClass.getMethod("sendSignal", String::class.java, String::class.java, String::class.java)
                    .invoke(mobileNode, sessionId, type, data)
                result.success(null)
            }
            "closeSignalSession" -> {
                checkNode()
                val sessionId = call.argument<String>("sessionId")!!
                mobileNode!!.javaClass.getMethod("closeSignalSession", String::class.java)
                    .invoke(mobileNode, sessionId)
                result.success(null)
            }
            "waitForPeer" -> {
                checkNode()
                val peerId = call.argument<String>("peerId")!!
                mobileNode!!.javaClass.getMethod("waitForPeer", String::class.java)
                    .invoke(mobileNode, peerId)
                result.success(null)
            }
            "close" -> {
                mobileNode?.javaClass?.getMethod("close")?.invoke(mobileNode)
                mobileNode = null
                handler = null
                result.success(null)
            }
            else -> result.notImplemented()
        }
    }

    private fun checkNode() {
        if (mobileNode == null) throw IllegalStateException("call createNode first")
    }

    private fun tryLoadBridge(): Class<*>? {
        return try {
            Class.forName("bridge.MobileNode")
        } catch (e: ClassNotFoundException) {
            null
        }
    }
}

class GoEventHandler(
    private val eventSink: EventChannel.EventSink?,
    private val scope: CoroutineScope,
) {
    fun onPeerDiscovered(peerId: String, addrs: String) {
        scope.launch {
            emit("peer_discovered", mapOf("peerId" to peerId, "addrs" to addrs))
        }
    }

    fun onSignalSession(sessionId: String, peerId: String) {
        scope.launch {
            emit("signal_session", mapOf("sessionId" to sessionId, "peerId" to peerId))
        }
    }

    fun onSignalMessage(sessionId: String, type: String, data: String) {
        scope.launch {
            emit("signal_message", mapOf(
                "sessionId" to sessionId, "type" to type, "data" to data
            ))
        }
    }

    fun onSignalSessionClosed(sessionId: String) {
        scope.launch {
            emit("signal_closed", mapOf("sessionId" to sessionId))
        }
    }

    fun onError(message: String) {
        scope.launch {
            emit("error", mapOf("message" to message))
        }
    }

    private suspend fun emit(event: String, data: Map<String, String?>) {
        val json = JSONObject().apply {
            put("event", event)
            data.forEach { (k, v) -> put(k, v ?: JSONObject.NULL) }
        }
        runCatching { eventSink?.success(json.toString()) }
    }
}
