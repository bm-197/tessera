import { requireNativeModule } from "expo-modules-core";

export interface TesseraCoreNativeModule {
  // Absolute path of the app's documents directory (where the vault lives).
  documentsPath(): string;
  // Dispatch a JSON command to the Go core; resolves with the JSON result,
  // rejects on a core error. See core/bridge for the method list.
  call(method: string, paramsJSON: string): Promise<string>;
  // Lock the vault and zero retained secrets.
  close(): void;
}

// Throws if the native module isn't linked — this app requires a dev/release
// build (expo prebuild), it cannot run in stock Expo Go.
export default requireNativeModule<TesseraCoreNativeModule>("TesseraCore");
