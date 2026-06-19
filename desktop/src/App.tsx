import { useEffect, useState } from "react";
import { api } from "./lib/client";
import Unlock from "./Unlock";
import Vault from "./Vault";

export default function App() {
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
        /* sidecar still starting — unlock screen is already shown */
      }
      try {
        setExists((await api.vaultExists()).exists);
      } catch {
        /* ignore */
      }
    })();
  }, []);

  if (unlocked) return <Vault onLock={() => setUnlocked(false)} />;
  return <Unlock exists={exists} onUnlocked={() => setUnlocked(true)} />;
}
