package expo.modules.tesseracore

import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition
import mobile.Client
import mobile.Mobile

class TesseraCoreModule : Module() {
  // One Go client for the lifetime of the app session.
  private val client: Client = Mobile.newClient()

  override fun definition() = ModuleDefinition {
    Name("TesseraCore")

    Function("documentsPath") {
      appContext.reactContext?.filesDir?.absolutePath ?: ""
    }

    // A Go error throws here and rejects the JS promise.
    AsyncFunction("call") { method: String, paramsJSON: String ->
      client.call(method, paramsJSON)
    }

    Function("close") {
      client.close()
    }
  }
}
