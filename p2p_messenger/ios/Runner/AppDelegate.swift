import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    GeneratedPluginRegistrant.register(with: self)
    P2pBridgePlugin.register(with: self.registrar(forPlugin: "P2pBridgePlugin")!)
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }
}
