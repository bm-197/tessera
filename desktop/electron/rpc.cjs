// JSON-RPC client over the tessera-sidecar Go process. One request per line on
// stdin, one response per line on stdout, correlated by id. This is the only
// place the desktop app touches the core; the renderer never sees the sidecar.
const { spawn } = require("child_process");
const readline = require("readline");

class SidecarClient {
  constructor(binPath) {
    this.proc = spawn(binPath, [], { stdio: ["pipe", "pipe", "inherit"] });
    this.nextId = 1;
    this.pending = new Map();

    this.rl = readline.createInterface({ input: this.proc.stdout });
    this.rl.on("line", (line) => {
      if (!line) return;
      let msg;
      try {
        msg = JSON.parse(line);
      } catch {
        return;
      }
      const p = this.pending.get(msg.id);
      if (!p) return;
      this.pending.delete(msg.id);
      if (msg.error) p.reject(new Error(msg.error));
      else p.resolve(msg.result);
    });

    this.proc.on("exit", (code) => {
      const err = new Error(`tessera-sidecar exited (code ${code})`);
      for (const p of this.pending.values()) p.reject(err);
      this.pending.clear();
    });
    this.proc.on("error", (err) => {
      for (const p of this.pending.values()) p.reject(err);
      this.pending.clear();
    });
  }

  call(method, params) {
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.proc.stdin.write(
        JSON.stringify({ id, method, params: params ?? {} }) + "\n"
      );
    });
  }

  close() {
    try {
      this.proc.stdin.end();
    } catch {}
    try {
      this.proc.kill();
    } catch {}
  }
}

module.exports = { SidecarClient };
