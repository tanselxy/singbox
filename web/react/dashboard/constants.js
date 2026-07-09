export const VIEW_META = {
  overview: { label: "总览", eyebrow: "Overview", title: "服务总览" },
  clients: { label: "客户管理", eyebrow: "Clients", title: "客户管理" },
  monitoring: { label: "实时监控", eyebrow: "Monitoring", title: "实时监控" },
  notifications: { label: "通知管理", eyebrow: "Notifications", title: "通知管理" },
  system: { label: "系统安全", eyebrow: "Security", title: "系统安全" },
  toolbox: { label: "工具箱", eyebrow: "Toolbox", title: "工具箱" },
  logs: { label: "运行日志", eyebrow: "Logs", title: "运行日志" },
};

export function routePath(view) {
  return view === "overview" ? "/dashboard" : `/dashboard/${view}`;
}
