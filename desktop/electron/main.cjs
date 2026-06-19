const { app, BrowserWindow, ipcMain, dialog, nativeImage } = require("electron");
const path = require("path");
const { SidecarClient } = require("./rpc.cjs");

app.setName("Tessera");

let win = null;
let client = null;

function sidecarPath() {
  const bin = process.platform === "win32" ? "tessera-sidecar.exe" : "tessera-sidecar";
  return app.isPackaged
    ? path.join(process.resourcesPath, "bin", bin)
    : path.join(__dirname, "bin", bin);
}

function vaultPath() {
  // Share the CLI/TUI vault. Go's os.UserConfigDir() resolves to the same
  // directory Electron exposes as "appData" on every platform (macOS:
  // ~/Library/Application Support, Linux: ~/.config, Windows: %AppData%).
  return path.join(app.getPath("appData"), "tessera", "default", "vault.json");
}

function createWindow() {
  win = new BrowserWindow({
    width: 440,
    height: 680,
    minWidth: 380,
    minHeight: 520,
    show: false, // reveal only once the first frame is painted (no white flash)
    backgroundColor: "#1b1e24", 
    autoHideMenuBar: true,
    titleBarStyle: "hiddenInset",
    webPreferences: {
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      webSecurity: true,
      backgroundThrottling: false,
      preload: path.join(__dirname, "preload.cjs"),
    },
  });

  let revealed = false;
  const reveal = () => {
    if (revealed || win.isDestroyed()) return;
    revealed = true;
    win.show();
    win.focus();
  };
  win.once("ready-to-show", reveal);
  win.webContents.once("did-finish-load", reveal);
  setTimeout(reveal, 1500);

  win.webContents.on("will-navigate", (e) => e.preventDefault());
  win.webContents.setWindowOpenHandler(() => ({ action: "deny" }));

  if (process.env.VITE_DEV_SERVER_URL) {
    win.loadURL(process.env.VITE_DEV_SERVER_URL);
  } else {
    win.loadFile(path.join(__dirname, "..", "dist", "index.html"));
  }
}

app.whenReady().then(() => {
  // Dev only: packaged builds get their icon from the bundle. This makes the
  // dock/app-switcher show the Tessera icon during `npm run dev` too.
  if (!app.isPackaged && process.platform === "darwin" && app.dock) {
    try {
      app.dock.setIcon(nativeImage.createFromPath(path.join(__dirname, "..", "build", "icon.png")));
    } catch {}
  }

  client = new SidecarClient(sidecarPath());

  ipcMain.handle("tessera:call", async (_e, payload) => {
    let { method, params } = payload || {};
    if (typeof method !== "string") {
      return { ok: false, error: "invalid call" };
    }
    if (method === "open" || method === "create" || method === "vaultExists") {
      params = { ...(params || {}), path: vaultPath() };
    }
    try {
      const data = await client.call(method, params || {});
      return { ok: true, data };
    } catch (err) {
      return { ok: false, error: err.message || String(err) };
    }
  });

  ipcMain.handle("tessera:pickSyncFile", async () => {
    const r = await dialog.showSaveDialog(win, {
      title: "Choose a sync file",
      defaultPath: "tessera-vault.blob",
    });
    return r.canceled ? null : r.filePath;
  });

  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

app.on("will-quit", () => {
  if (client) client.close(); // closing stdin locks the vault in the core
});
