// Conexão com o servidor. O nginx repassa /ws para o container do Go,
// então o endereço é sempre o mesmo host que serviu a página.
export class Net {
  constructor() {
    this.ws = null;
    this.handlers = {};
  }

  on(type, fn) {
    this.handlers[type] = fn;
    return this;
  }

  connect(name, weapon = 0) {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    this.ws = new WebSocket(`${proto}://${location.host}/ws?name=${encodeURIComponent(name)}&arma=${weapon}`);
    this.ws.onopen = () => this.handlers.open?.();
    this.ws.onclose = () => this.handlers.close?.();
    this.ws.onmessage = (ev) => {
      let msg;
      try { msg = JSON.parse(ev.data); } catch { return; }
      this.handlers[msg.t]?.(msg);
    };
  }

  send(obj) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(obj));
  }

  close() {
    this.ws?.close();
  }
}
