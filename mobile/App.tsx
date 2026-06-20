import { useEffect, useState } from "react";
import { StatusBar } from "expo-status-bar";
import { View } from "react-native";
import { api } from "./src/lib/client";
import { C } from "./src/lib/theme";
import Unlock from "./src/screens/Unlock";
import Vault from "./src/screens/Vault";

export default function App() {
  // Render the unlock screen immediately; resolve vault state in the background.
  const [unlocked, setUnlocked] = useState(false);
  const [exists, setExists] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        if ((await api.status()).unlocked) {
          setUnlocked(true);
          return;
        }
      } catch {
        /* native module/core not ready */
      }
      try {
        setExists((await api.vaultExists()).exists);
      } catch {
        /* ignore */
      }
    })();
  }, []);

  return (
    <View style={{ flex: 1, backgroundColor: C.bg }}>
      <StatusBar style="light" />
      {unlocked ? (
        <Vault onLock={() => setUnlocked(false)} />
      ) : (
        <Unlock exists={exists} onUnlocked={() => setUnlocked(true)} />
      )}
    </View>
  );
}
