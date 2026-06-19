// Headless smoke test: boots the real renderer + preload in a hidden Electron
// window, wired to the Go sidecar, and asserts that window.tessera is exposed
// and the app renders. Exits 0 on success, 1 on failure. No window is shown.
const { app, BrowserWindow, ipcMain } = require("electron");
const path = require("path");
const os = require("os");
const fs = require("fs");
const { SidecarClient } = require("../electron/rpc.cjs");

const TIMEOUT_MS = 15000;
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "tessera-smoke-"));
let client = null;

function cleanup() {
  try {
    client && client.close();
  } catch {}
}
function fail(msg) {
  console.error("SMOKE_FAIL:", msg);
  cleanup();
  app.exit(1);
}
function pass() {
  console.log("SMOKE_OK");
  cleanup();
  app.exit(0);
}

if (process.platform === "darwin" && app.dock) app.dock.hide();

app.whenReady().then(() => {
  client = new SidecarClient(path.join(__dirname, "..", "electron", "bin", "tessera-sidecar"));
  const vaultPath = path.join(tmp, "vault.json");

  ipcMain.handle("tessera:call", async (_e, { method, params }) => {
    if (method === "open" || method === "create" || method === "vaultExists") {
      params = { ...(params || {}), path: vaultPath };
    }
    try {
      return { ok: true, data: await client.call(method, params || {}) };
    } catch (e) {
      return { ok: false, error: e.message };
    }
  });
  ipcMain.handle("tessera:pickSyncFile", async () => null);

  const win = new BrowserWindow({
    show: false,
    webPreferences: {
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      preload: path.join(__dirname, "..", "electron", "preload.cjs"),
    },
  });

  win.webContents.on("did-fail-load", (_e, _c, desc) => fail("did-fail-load: " + desc));
  win.webContents.on("render-process-gone", (_e, d) => fail("render-process-gone: " + d.reason));

  const timer = setTimeout(() => fail("timeout"), TIMEOUT_MS);

  win.webContents.once("did-finish-load", async () => {
    await new Promise((r) => setTimeout(r, 1000)); // let async status/exists run
    try {
      const bridge = await win.webContents.executeJavaScript("typeof window.tessera");
      const rendered = await win.webContents.executeJavaScript(
        "document.getElementById('root').innerHTML.length"
      );
      clearTimeout(timer);
      if (bridge !== "object") return fail("window.tessera not exposed (" + bridge + ")");
      if (!rendered) return fail("renderer rendered nothing");
      pass();
    } catch (e) {
      clearTimeout(timer);
      fail("executeJavaScript: " + e.message);
    }
  });

  win.loadFile(path.join(__dirname, "..", "dist", "index.html"));
});

app.on("window-all-closed", () => {});
