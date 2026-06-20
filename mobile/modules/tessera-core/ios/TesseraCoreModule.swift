import ExpoModulesCore
import TesseraGo // the gomobile-bound xcframework (Go symbols are Mobile*)
// Named TesseraGo (not Tessera) to avoid colliding with the app's own module.

public class TesseraCoreModule: Module {
  // One Go client for the lifetime of the app session.
  private let client = MobileNew()

  public func definition() -> ModuleDefinition {
    Name("TesseraCore")

    Function("documentsPath") { () -> String in
      let dir = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
      return dir?.path ?? NSTemporaryDirectory()
    }

    // gomobile gives the bound method a _Nonnull return, so Swift keeps the
    // trailing NSError** as an explicit out-parameter rather than `throws`.
    // Convert it back: surface a core error as a thrown error (rejects the JS
    // promise), otherwise return the JSON result.
    AsyncFunction("call") { (method: String, paramsJSON: String) throws -> String in
      var error: NSError?
      let result = self.client?.call(method, paramsJSON: paramsJSON, error: &error)
      if let error = error { throw error }
      return result ?? ""
    }

    Function("close") {
      self.client?.close()
    }
  }
}
