// Node link -> Clash proxy fragment converter. Ported from the legacy
// vless-clash.html tool. Runs entirely client-side.
(function () {
  "use strict";

  function parseVless(url) {
    const u = new URL(url);
    const p = new URLSearchParams(u.search);
    const cfg = {
      name: decodeURIComponent(u.hash.substring(1)) || "VLESS节点",
      type: "vless",
      server: u.hostname,
      port: parseInt(u.port),
      uuid: u.username,
      network: p.get("type") || "tcp",
      tls: p.get("security") === "tls" || p.get("security") === "reality",
      udp: true,
    };
    if (p.get("flow")) cfg.flow = p.get("flow");
    if (p.get("sni")) cfg.servername = p.get("sni");
    if (p.get("fp")) cfg["client-fingerprint"] = p.get("fp");
    if (p.get("security") === "reality") {
      cfg["reality-opts"] = {};
      if (p.get("pbk")) cfg["reality-opts"]["public-key"] = p.get("pbk");
      if (p.get("sid")) cfg["reality-opts"]["short-id"] = p.get("sid");
    }
    if (p.get("type") === "ws") {
      cfg["ws-opts"] = {};
      if (p.get("path")) cfg["ws-opts"].path = p.get("path");
      if (p.get("host")) cfg["ws-opts"].headers = { Host: p.get("host") };
    }
    if (p.get("type") === "grpc") {
      cfg["grpc-opts"] = {};
      if (p.get("serviceName")) cfg["grpc-opts"]["grpc-service-name"] = p.get("serviceName");
    }
    return cfg;
  }

  function parseVmess(url) {
    const o = JSON.parse(atob(url.replace("vmess://", "")));
    const cfg = {
      name: o.ps || "VMess节点",
      type: "vmess",
      server: o.add,
      port: parseInt(o.port),
      uuid: o.id,
      alterId: parseInt(o.aid) || 0,
      cipher: o.scy || "auto",
      udp: true,
    };
    if (o.tls === "tls") {
      cfg.tls = true;
      if (o.sni) cfg.servername = o.sni;
    }
    if (o.net === "ws") {
      cfg.network = "ws";
      cfg["ws-opts"] = { path: o.path || "/" };
      if (o.host) cfg["ws-opts"].headers = { Host: o.host };
    }
    return cfg;
  }

  function parseTrojan(url) {
    const u = new URL(url);
    const p = new URLSearchParams(u.search);
    const cfg = {
      name: decodeURIComponent(u.hash.substring(1)) || "Trojan节点",
      type: "trojan",
      server: u.hostname,
      port: parseInt(u.port),
      password: u.username,
      udp: true,
    };
    if (p.get("sni")) cfg.sni = p.get("sni");
    if (p.get("type") === "ws") {
      cfg.network = "ws";
      cfg["ws-opts"] = {};
      if (p.get("path")) cfg["ws-opts"].path = p.get("path");
      if (p.get("host")) cfg["ws-opts"].headers = { Host: p.get("host") };
    }
    return cfg;
  }

  function toYaml(obj, indent) {
    indent = indent || 0;
    let out = "";
    const pad = "  ".repeat(indent);
    for (const [k, v] of Object.entries(obj)) {
      if (v === null || v === undefined) continue;
      if (typeof v === "object" && !Array.isArray(v)) {
        out += pad + k + ":\n" + toYaml(v, indent + 1);
      } else {
        out += pad + k + ": " + v + "\n";
      }
    }
    return out;
  }

  function message(text, kind) {
    const m = document.getElementById("message");
    m.className = "text-sm " + (kind === "alert" ? "text-destructive" : "text-success");
    m.textContent = text;
    setTimeout(() => {
      m.textContent = "";
      m.className = "text-sm";
    }, 3000);
  }

  function convert() {
    const input = document.getElementById("input").value.trim();
    const output = document.getElementById("output");
    if (!input) return message("请输入节点链接", "alert");
    try {
      let cfg;
      if (input.startsWith("vless://")) cfg = parseVless(input);
      else if (input.startsWith("vmess://")) cfg = parseVmess(input);
      else if (input.startsWith("trojan://")) cfg = parseTrojan(input);
      else throw new Error("不支持的协议类型");
      output.value = "proxies:\n  - " + toYaml(cfg, 2).trimStart();
      message("转换成功", "ok-msg");
    } catch (e) {
      output.value = "";
      message("转换失败: " + e.message, "alert");
    }
  }

  document.getElementById("convert").addEventListener("click", convert);
  document.getElementById("clear").addEventListener("click", () => {
    document.getElementById("input").value = "";
    document.getElementById("output").value = "";
  });
  document.getElementById("copy-out").addEventListener("click", () => {
    const o = document.getElementById("output");
    if (!o.value) return message("没有可复制的内容", "alert");
    navigator.clipboard.writeText(o.value).then(() => message("已复制", "ok-msg"));
  });
  document.getElementById("input").addEventListener("keydown", (e) => {
    if (e.ctrlKey && e.key === "Enter") convert();
  });
})();
