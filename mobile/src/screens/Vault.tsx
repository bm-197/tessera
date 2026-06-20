import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActivityIndicator,
  Alert,
  Animated,
  FlatList,
  Modal,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import * as Clipboard from "expo-clipboard";
import { api, CodeView, displayName, formatCode } from "../lib/client";
import { C, MONO } from "../lib/theme";

function Bar({ ms, period }: { ms: number; period: number }) {
  const frac = Math.max(0, Math.min(1, ms / (period * 1000)));
  const low = ms <= 5000;
  return (
    <View style={s.barTrack}>
      <View style={[s.barFill, { width: `${frac * 100}%`, backgroundColor: low ? C.yellow : C.green }]} />
    </View>
  );
}

export default function Vault({ onLock }: { onLock: () => void }) {
  const [codes, setCodes] = useState<CodeView[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [adding, setAdding] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);

  const [toast, setToast] = useState<string | null>(null);
  const toastOpacity = useRef(new Animated.Value(0)).current;
  const flash = useCallback(
    (msg: string) => {
      setToast(msg);
      Animated.timing(toastOpacity, { toValue: 1, duration: 140, useNativeDriver: true }).start();
      setTimeout(() => {
        Animated.timing(toastOpacity, { toValue: 0, duration: 160, useNativeDriver: true }).start(
          ({ finished }) => finished && setToast(null)
        );
      }, 1300);
    },
    [toastOpacity]
  );

  const refresh = useCallback(async () => {
    try {
      setCodes((await api.codes()).codes);
      setLoaded(true);
    } catch {
      /* locked/transient */
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 1000);
    return () => clearInterval(id);
  }, [refresh]);

  async function copy(c: CodeView) {
    if (c.error) return;
    await Clipboard.setStringAsync(c.code);
    flash("Copied");
  }

  async function lock() {
    await api.lock();
    onLock();
  }

  const liveDetail = detailId ? codes.find((c) => c.id === detailId) ?? null : null;

  return (
    <View style={s.app}>
      <View style={s.header}>
        <Text style={s.title}>Tessera</Text>
        <View style={s.headerActions}>
          <Pressable style={s.iconbtn} onPress={() => setAdding(true)}>
            <Text style={s.iconbtnText}>+ Add</Text>
          </Pressable>
          <Pressable style={s.iconbtn} onPress={lock}>
            <Text style={s.iconbtnText}>Lock</Text>
          </Pressable>
        </View>
      </View>

      {!loaded ? (
        <View style={s.center}>
          <ActivityIndicator color={C.muted} />
        </View>
      ) : codes.length === 0 ? (
        <View style={s.center}>
          <Text style={s.empty}>No accounts yet.{"\n"}Tap “+ Add” to add one.</Text>
        </View>
      ) : (
        <FlatList
          data={codes}
          keyExtractor={(c) => c.id}
          contentContainerStyle={s.list}
          renderItem={({ item: c }) => (
            <Pressable
              style={({ pressed }) => [s.card, pressed && s.cardPressed]}
              onPress={() => copy(c)}
            >
              <View style={s.cardInfo}>
                <Text style={s.cardName} numberOfLines={1}>
                  {c.issuer || c.label || "Unnamed"}
                </Text>
                {(!!(c.issuer && c.label) || c.recoveryCount > 0) && (
                  <Text style={s.cardSub} numberOfLines={1}>
                    {c.issuer && c.label ? c.label : ""}
                    {c.recoveryCount > 0 ? `   🔑 ${c.recoveryCount}` : ""}
                  </Text>
                )}
                {!c.error && (
                  <View style={s.barRow}>
                    <Bar ms={c.expiresInMs} period={c.period} />
                  </View>
                )}
              </View>
              <Text style={[s.code, !!c.error && s.codeErr]}>
                {c.error ? "error" : formatCode(c.code)}
              </Text>
              <Pressable style={s.kebab} hitSlop={12} onPress={() => setDetailId(c.id)}>
                <Text style={s.kebabText}>⋯</Text>
              </Pressable>
            </Pressable>
          )}
        />
      )}

      <Modal visible={adding} animationType="slide" transparent onRequestClose={() => setAdding(false)}>
        <AddSheet
          onClose={() => setAdding(false)}
          onAdded={() => {
            setAdding(false);
            refresh();
            flash("Added");
          }}
        />
      </Modal>

      <Modal
        visible={!!liveDetail}
        animationType="slide"
        transparent
        onRequestClose={() => setDetailId(null)}
      >
        {liveDetail && (
          <DetailSheet
            acct={liveDetail}
            onClose={() => setDetailId(null)}
            onCopy={() => copy(liveDetail)}
            onChanged={(msg) => {
              refresh();
              flash(msg);
            }}
            onDeleted={() => {
              setDetailId(null);
              refresh();
              flash("Deleted");
            }}
          />
        )}
      </Modal>

      {toast && (
        <Animated.View style={[s.toast, { opacity: toastOpacity }]} pointerEvents="none">
          <Text style={s.toastText}>{toast}</Text>
        </Animated.View>
      )}
    </View>
  );
}

function field(label: string, value: string, onChange: (t: string) => void, multiline = false) {
  return (
    <View>
      <Text style={s.fieldLabel}>{label}</Text>
      <TextInput
        style={[s.input, multiline && s.multiline]}
        value={value}
        onChangeText={onChange}
        autoCapitalize="none"
        autoCorrect={false}
        multiline={multiline}
        placeholderTextColor={C.dim}
      />
    </View>
  );
}

function DetailSheet({
  acct,
  onClose,
  onCopy,
  onChanged,
  onDeleted,
}: {
  acct: CodeView;
  onClose: () => void;
  onCopy: () => void;
  onChanged: (msg: string) => void;
  onDeleted: () => void;
}) {
  const [issuer, setIssuer] = useState(acct.issuer);
  const [label, setLabel] = useState(acct.label);
  const [recovery, setRecovery] = useState("");
  const [savingName, setSavingName] = useState(false);
  const [savingRec, setSavingRec] = useState(false);

  useEffect(() => {
    api
      .recoveryCodes(acct.id)
      .then((r) => setRecovery(r.codes.join("\n")))
      .catch(() => {});
  }, [acct.id]);

  async function saveName() {
    if (!issuer.trim()) return;
    setSavingName(true);
    try {
      await api.setName(acct.id, issuer.trim(), label.trim());
      onChanged("Renamed");
    } finally {
      setSavingName(false);
    }
  }

  async function saveRecovery() {
    setSavingRec(true);
    try {
      await api.setRecoveryCodes(
        acct.id,
        recovery.split("\n").map((l) => l.trim()).filter(Boolean)
      );
      onChanged("Recovery saved");
    } finally {
      setSavingRec(false);
    }
  }

  function confirmDelete() {
    Alert.alert("Delete account", `Delete ${displayName(acct)}?`, [
      { text: "Cancel", style: "cancel" },
      { text: "Delete", style: "destructive", onPress: () => api.remove(acct.id).then(onDeleted) },
    ]);
  }

  return (
    <View style={s.sheetWrap}>
      <View style={s.sheet}>
        <Text style={s.sheetTitle}>{issuer || label || "Unnamed"}</Text>

        <Pressable style={s.codeRow} onPress={onCopy}>
          <Text style={s.bigCode}>{acct.error ? "error" : formatCode(acct.code)}</Text>
          <Text style={s.copyHint}>tap to copy</Text>
        </Pressable>

        {field("Account name", issuer, setIssuer)}
        {field("Label", label, setLabel)}
        <View style={s.rowEnd}>
          <Pressable style={[s.btn, s.btnSecondary]} onPress={saveName} disabled={savingName}>
            {savingName ? <ActivityIndicator color={C.muted} /> : <Text style={s.btnSecondaryText}>Save name</Text>}
          </Pressable>
        </View>

        {field("Recovery codes (one per line)", recovery, setRecovery, true)}
        <View style={s.rowEnd}>
          <Pressable style={[s.btn, s.btnSecondary]} onPress={saveRecovery} disabled={savingRec}>
            {savingRec ? <ActivityIndicator color={C.muted} /> : <Text style={s.btnSecondaryText}>Save codes</Text>}
          </Pressable>
        </View>

        <View style={s.sheetFooter}>
          <Pressable style={[s.btn, s.btnDanger]} onPress={confirmDelete}>
            <Text style={s.btnText}>Delete</Text>
          </Pressable>
          <Pressable style={[s.btn, s.btnSecondary]} onPress={onClose}>
            <Text style={s.btnSecondaryText}>Close</Text>
          </Pressable>
        </View>
      </View>
    </View>
  );
}

function AddSheet({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const [secret, setSecret] = useState("");
  const [name, setName] = useState("");
  const [label, setLabel] = useState("");
  const [recovery, setRecovery] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const lines = (str: string) => str.split("\n").map((l) => l.trim()).filter(Boolean);
  const isURI = secret.trim().startsWith("otpauth://");

  async function submit() {
    setErr("");
    const raw = secret.trim();
    if (!raw) return setErr("Enter a secret or otpauth:// URI.");
    setBusy(true);
    try {
      if (isURI) {
        await api.addURI(raw, lines(recovery));
      } else {
        if (!name.trim()) {
          setErr("Account name is required.");
          setBusy(false);
          return;
        }
        await api.addManual({ issuer: name.trim(), label: label.trim(), secret: raw, recoveryCodes: lines(recovery) });
      }
      onAdded();
    } catch (e) {
      setErr(String((e as Error).message ?? e));
      setBusy(false);
    }
  }

  return (
    <View style={s.sheetWrap}>
      <View style={s.sheet}>
        <Text style={s.sheetTitle}>Add account</Text>
        {field("Secret or otpauth:// URI", secret, setSecret, true)}
        {!isURI && field("Account name (required)", name, setName)}
        {!isURI && field("Label (optional)", label, setLabel)}
        {field("Recovery codes (optional, one per line)", recovery, setRecovery, true)}
        {!!err && <Text style={s.err}>{err}</Text>}
        <View style={s.rowEnd}>
          <Pressable style={[s.btn, s.btnSecondary]} onPress={onClose} disabled={busy}>
            <Text style={s.btnSecondaryText}>Cancel</Text>
          </Pressable>
          <Pressable style={[s.btn, busy && { opacity: 0.55 }]} onPress={submit} disabled={busy}>
            {busy ? <ActivityIndicator color={C.ink} /> : <Text style={s.btnText}>Add</Text>}
          </Pressable>
        </View>
      </View>
    </View>
  );
}

const s = StyleSheet.create({
  app: { flex: 1, backgroundColor: C.bg },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 16,
    paddingTop: 60,
    paddingBottom: 12,
    borderBottomWidth: 1,
    borderBottomColor: C.border,
  },
  title: { color: C.purple, fontFamily: MONO, fontSize: 17, fontWeight: "700" },
  headerActions: { flexDirection: "row", gap: 8 },
  iconbtn: { borderWidth: 1, borderColor: C.border, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 6 },
  iconbtnText: { color: C.muted, fontSize: 13 },

  center: { flex: 1, alignItems: "center", justifyContent: "center", padding: 24 },
  empty: { color: C.dim, textAlign: "center", fontFamily: MONO, lineHeight: 22 },

  list: { padding: 14, gap: 12 },
  card: {
    backgroundColor: C.panel,
    borderWidth: 1,
    borderColor: C.border,
    borderRadius: 12,
    padding: 14,
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
  },
  cardPressed: { backgroundColor: C.panel2 },
  cardInfo: { flex: 1, minWidth: 0 },
  cardName: { color: C.text, fontWeight: "600", fontSize: 15 },
  cardSub: { color: C.dim, fontSize: 12, marginTop: 2 },
  barRow: { marginTop: 8 },
  barTrack: { height: 4, borderRadius: 2, backgroundColor: C.border, overflow: "hidden" },
  barFill: { height: 4, borderRadius: 2 },
  code: { color: C.green, fontFamily: MONO, fontSize: 25, fontWeight: "700", letterSpacing: 1 },
  codeErr: { color: C.red, fontSize: 14 },
  kebab: { paddingHorizontal: 4, paddingVertical: 2 },
  kebabText: { color: C.dim, fontSize: 20, lineHeight: 20 },

  sheetWrap: { flex: 1, justifyContent: "flex-end", backgroundColor: "rgba(0,0,0,0.55)" },
  sheet: {
    backgroundColor: C.panel,
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
    borderWidth: 1,
    borderColor: C.border,
    padding: 18,
    paddingBottom: 34,
    gap: 12,
  },
  sheetTitle: { color: C.purple, fontFamily: MONO, fontSize: 16, fontWeight: "700" },
  sheetFooter: { flexDirection: "row", justifyContent: "space-between", marginTop: 6 },
  rowEnd: { flexDirection: "row", justifyContent: "flex-end", gap: 10 },

  codeRow: { alignItems: "center", paddingVertical: 8 },
  bigCode: { color: C.green, fontFamily: MONO, fontSize: 40, fontWeight: "700", letterSpacing: 2 },
  copyHint: { color: C.dim, fontSize: 12, marginTop: 4 },

  fieldLabel: { color: C.muted, fontSize: 12, marginBottom: 5 },
  input: {
    backgroundColor: C.bg,
    borderWidth: 1,
    borderColor: C.border,
    color: C.text,
    fontFamily: MONO,
    fontSize: 14,
    paddingHorizontal: 11,
    paddingVertical: 10,
    borderRadius: 10,
  },
  multiline: { minHeight: 60, textAlignVertical: "top" },
  err: { color: C.red, fontSize: 13 },

  btn: { backgroundColor: C.green, borderRadius: 10, paddingHorizontal: 18, paddingVertical: 11, alignItems: "center", justifyContent: "center", minWidth: 96 },
  btnText: { color: C.ink, fontWeight: "700" },
  btnSecondary: { backgroundColor: "transparent", borderWidth: 1, borderColor: C.border },
  btnSecondaryText: { color: C.muted },
  btnDanger: { backgroundColor: C.red },

  toast: {
    position: "absolute",
    bottom: 40,
    alignSelf: "center",
    backgroundColor: C.panel2,
    borderWidth: 1,
    borderColor: C.border,
    paddingHorizontal: 16,
    paddingVertical: 9,
    borderRadius: 10,
  },
  toastText: { color: C.text, fontFamily: MONO, fontSize: 13 },
});
