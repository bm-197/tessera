import { useState } from "react";
import {
  ActivityIndicator,
  Image,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { api } from "../lib/client";
import { C, MONO } from "../lib/theme";

const icon = require("../assets/icon.png");

export default function Unlock({
  exists,
  onUnlocked,
}: {
  exists: boolean;
  onUnlocked: () => void;
}) {
  const [pass, setPass] = useState("");
  const [confirm, setConfirm] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    setErr("");
    if (!pass) return;
    if (!exists && pass !== confirm) {
      setErr("passphrases do not match");
      return;
    }
    setBusy(true);
    try {
      if (exists) await api.open(pass);
      else await api.create(pass);
      onUnlocked();
    } catch (e) {
      setErr(exists ? "wrong passphrase" : String((e as Error).message ?? e));
      setBusy(false);
    }
  }

  return (
    <KeyboardAvoidingView
      style={s.wrap}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <View style={s.inner}>
        <Image source={icon} style={s.logo} />
        <Text style={s.title}>Tessera</Text>
        <Text style={s.prompt}>
          {exists
            ? "Enter your passphrase to unlock."
            : "Create a vault. Your passphrase has no recovery, so keep it safe."}
        </Text>

        <TextInput
          style={s.input}
          placeholder="passphrase"
          placeholderTextColor={C.dim}
          secureTextEntry
          autoFocus
          editable={!busy}
          value={pass}
          onChangeText={setPass}
          onSubmitEditing={exists ? submit : undefined}
        />
        {!exists && (
          <TextInput
            style={s.input}
            placeholder="confirm passphrase"
            placeholderTextColor={C.dim}
            secureTextEntry
            editable={!busy}
            value={confirm}
            onChangeText={setConfirm}
            onSubmitEditing={submit}
          />
        )}

        <Text style={s.err}>{err}</Text>

        <Pressable
          style={[s.btn, busy && s.btnDisabled]}
          onPress={submit}
          disabled={busy || !pass}
        >
          {busy ? (
            <ActivityIndicator color={C.ink} />
          ) : (
            <Text style={s.btnText}>{exists ? "Unlock" : "Create vault"}</Text>
          )}
        </Pressable>
      </View>
    </KeyboardAvoidingView>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, justifyContent: "center", padding: 24 },
  inner: { alignItems: "center", gap: 14, width: "100%", maxWidth: 320, alignSelf: "center" },
  logo: { width: 64, height: 64, borderRadius: 16 },
  title: { color: C.purple, fontFamily: MONO, fontSize: 22, fontWeight: "700" },
  prompt: { color: C.muted, textAlign: "center", fontSize: 14, lineHeight: 20 },
  input: {
    width: "100%",
    backgroundColor: C.bg,
    borderWidth: 1,
    borderColor: C.border,
    color: C.text,
    fontFamily: MONO,
    fontSize: 15,
    paddingHorizontal: 12,
    paddingVertical: 12,
    borderRadius: 10,
  },
  err: { color: C.red, fontSize: 13, minHeight: 18 },
  btn: {
    width: "100%",
    backgroundColor: C.green,
    borderRadius: 10,
    paddingVertical: 13,
    alignItems: "center",
    justifyContent: "center",
  },
  btnDisabled: { opacity: 0.55 },
  btnText: { color: C.ink, fontWeight: "700", fontSize: 15 },
});
