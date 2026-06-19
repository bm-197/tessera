const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("tessera", {
  call: (method, params) => ipcRenderer.invoke("tessera:call", { method, params }),
  pickSyncFile: () => ipcRenderer.invoke("tessera:pickSyncFile"),
});
