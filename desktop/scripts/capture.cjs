// Visual-QA capture: renders the real built UI in a hidden Electron window
// (wired to a throwaway vault) and saves PNGs of each state via capturePage.
// No visible window appears. Usage: electron scripts/capture.cjs
const { app, BrowserWindow, ipcMain } = require("electron");
const path = require("path");
const os = require("os");
const fs = require("fs");
const { SidecarClient } = require("../electron/rpc.cjs");

const OUT = "/tmp/tessera-shots";
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "tessera-cap-"));
const vaultPath = path.join(tmp, "vault.json");
let client;

const wait = (ms) => new Promise((r) => setTimeout(r, ms));

async function shot(win, name) {
  const img = await win.webContents.capturePage();
  fs.writeFileSync(path.join(OUT, name + ".png"), img.toPNG());
  console.log("shot:", name);
}

app.whenReady().then(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  if (process.platform === "darwin" && app.dock) app.dock.hide();
  client = new SidecarClient(path.join(__dirname, "..", "electron", "bin", "tessera-sidecar"));

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

  // Seed a vault, then lock so the first window shows the unlock screen.
  await client.call("create", { path: vaultPath, passphrase: "demo" });
  await client.call("addURI", {
    uri: "otpauth://totp/GitHub:bm-197?secret=JBSWY3DPEHPK3PXP&issuer=GitHub",
    recoveryCodes: [],
  });
  await client.call("addManual", {
    issuer: "Stripe",
    label: "bisrat.maru-ug@aau.edu.et",
    secret: "JBSWY3DPEHPK3PXP",
    recoveryCodes: ["zwdg-bywm-btgv"],
  });
  await client.call("addURI", {
    uri: "otpauth://totp/Fastmail:you@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Fastmail",
    recoveryCodes: [],
  });
  await client.call("lock", {});

  const win = new BrowserWindow({
    width: 440,
    height: 680,
    show: false,
    webPreferences: {
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      preload: path.join(__dirname, "..", "electron", "preload.cjs"),
    },
  });

  await win.loadFile(path.join(__dirname, "..", "dist", "index.html"));
  await wait(700);
  await shot(win, "1-unlock");

  // Unlock in the core, reload → list state.
  await client.call("open", { path: vaultPath, passphrase: "demo" });
  await win.reload();
  await wait(1300);
  await shot(win, "2-list");

  // Tooltip (focus the sync button; :focus-within shows the same tooltip as hover).
  await win.webContents.executeJavaScript("document.querySelectorAll('.iconbtn')[0]?.focus()");
  await wait(300);
  await shot(win, "2b-tooltip");
  await win.webContents.executeJavaScript("document.activeElement?.blur()");
  await wait(150);

  // Add modal.
  await win.webContents.executeJavaScript("document.querySelector('.fab')?.click()");
  await wait(450);
  await shot(win, "3-add");
  await win.webContents.executeJavaScript("document.querySelector('.overlay')?.click()");
  await wait(300);

  // Detail modal.
  await win.webContents.executeJavaScript("document.querySelector('.kebab')?.click()");
  await wait(600);
  await shot(win, "4-detail");

  client.close();
  app.exit(0);
});

app.on("window-all-closed", () => {});
