const {
    app,
    BrowserWindow,
    session,
    ipcMain,
    BrowserView,
    Menu,
    MenuItem,
    clipboard,
} = require("electron");
const { spawn } = require("child_process");
const path = require("path");

let mainWindow = null;
let tunnelProc = null;
let tunnelPort = null;

// ========= 隧道配置 =========
const TUNNEL_TRANSPORT = "quic"; // http2 / quic / ssh2
const AUTO_DISCOVER = true; // true=组播自动发现（推荐）
const PROTO_MODE = "socks5";
const DISCOVER_TIMEOUT = 5;
// ============================

const tabs = [];
let activeTabId = null;
let tabIdSeq = 1;

function createWindow() {
    mainWindow = new BrowserWindow({
        width: 1280,
        height: 800,
        webPreferences: {
            nodeIntegration: true,
            contextIsolation: false,
        },
    });
    mainWindow.loadFile("index.html");
    mainWindow.on("resize", layoutViews);
}

function layoutViews() {
    if (!mainWindow) return;
    const [w, h] = mainWindow.getContentSize();
    const viewY = 140;
    const viewH = Math.max(100, h - viewY);
    for (const t of tabs) {
        if (t.view) {
            t.view.setBounds({ x: 0, y: viewY, width: w, height: viewH });
        }
    }
}

function createTab(url = "https://www.baidu.com") {
    const tabId = tabIdSeq++;
    const view = new BrowserView({
        webPreferences: { session: session.defaultSession },
    });
    mainWindow.addBrowserView(view);

    const tab = { id: tabId, url, title: "加载中...", view };
    tabs.push(tab);

    view.webContents.setWindowOpenHandler((details) => {
        createTab(details.url);
        return { action: "deny" };
    });

    view.webContents.on("context-menu", (e, params) => {
        const menu = new Menu();
        if (params.linkURL) {
            menu.append(
                new MenuItem({
                    label: "在新标签打开链接",
                    click: () => createTab(params.linkURL),
                })
            );
            menu.append(
                new MenuItem({
                    label: "复制链接地址",
                    click: () => clipboard.writeText(params.linkURL),
                })
            );
            menu.append(new MenuItem({ type: "separator" }));
        }
        if (params.selectionText) {
            menu.append(
                new MenuItem({
                    label: "复制",
                    click: () => clipboard.writeText(params.selectionText),
                })
            );
            menu.append(new MenuItem({ type: "separator" }));
        }
        menu.append(
            new MenuItem({
                label: "后退",
                click: () => view.webContents.goBack(),
            })
        );
        menu.append(
            new MenuItem({
                label: "前进",
                click: () => view.webContents.goForward(),
            })
        );
        menu.append(
            new MenuItem({
                label: "刷新",
                click: () => view.webContents.reload(),
            })
        );
        menu.append(new MenuItem({ type: "separator" }));
        menu.append(
            new MenuItem({
                label: "查看网页源码",
                click: () => view.webContents.showDefinitionForSelection(),
            })
        );
        menu.append(
            new MenuItem({
                label: "检查元素(F12)",
                click: () => view.webContents.toggleDevTools(),
            })
        );
        menu.popup();
    });

    view.webContents.on("page-title-updated", (evt, title) => {
        tab.title = title;
        mainWindow.webContents.send("page-nav", { tabId, url: tab.url, title });
        sendTabUpdate();
    });
    view.webContents.on("did-navigate", (evt, newUrl) => {
        tab.url = newUrl;
        mainWindow.webContents.send("page-nav", {
            tabId,
            url: newUrl,
            title: tab.title,
        });
        sendTabUpdate();
        if (activeTabId === tabId)
            mainWindow.webContents.send("active-url", newUrl);
    });
    view.webContents.on("did-navigate-in-page", (evt, newUrl) => {
        tab.url = newUrl;
        mainWindow.webContents.send("page-nav", {
            tabId,
            url: newUrl,
            title: tab.title,
        });
        sendTabUpdate();
        if (activeTabId === tabId)
            mainWindow.webContents.send("active-url", newUrl);
    });

    view.webContents.loadURL(url);
    setActiveTab(tabId);
    sendTabUpdate();
    return tabId;
}

function setActiveTab(tabId) {
    const tab = tabs.find((t) => t.id === tabId);
    if (!tab) return;
    activeTabId = tabId;
    for (const t of tabs) {
        if (t.view) t.view.setVisible(t.id === tabId);
    }
    layoutViews();
    sendTabUpdate();
    mainWindow.webContents.send("active-url", tab.url);
}

function closeTab(tabId) {
    const idx = tabs.findIndex((t) => t.id === tabId);
    if (idx === -1) return false;
    const tab = tabs[idx];
    if (tab.view) {
        mainWindow.removeBrowserView(tab.view);
        tab.view.webContents.destroy();
    }
    tabs.splice(idx, 1);
    if (activeTabId === tabId) {
        if (tabs.length > 0) {
            const newActive = tabs[Math.max(0, idx - 1)];
            setActiveTab(newActive.id);
        } else {
            activeTabId = null;
        }
    }
    sendTabUpdate();
    return true;
}

function getActiveTab() {
    return tabs.find((t) => t.id === activeTabId);
}
function sendTabUpdate() {
    mainWindow.webContents.send(
        "tab-update",
        tabs.map((t) => ({
            id: t.id,
            title: t.title,
            url: t.url,
            active: t.id === activeTabId,
        }))
    );
}

ipcMain.handle("new-tab", (_, url) =>
    createTab(url || "https://www.baidu.com")
);
ipcMain.handle("switch-tab", (_, tabId) => setActiveTab(tabId));
ipcMain.handle("close-tab", (_, tabId) => closeTab(tabId));
ipcMain.handle("navigate", (_, url) => {
    const tab = getActiveTab();
    if (!tab) return;
    let u = url.trim();
    if (!u.startsWith("http")) u = "https://" + u;
    tab.view.webContents.loadURL(u);
});
ipcMain.handle("go-back", () => {
    const t = getActiveTab();
    if (t) t.view.webContents.goBack();
});
ipcMain.handle("go-forward", () => {
    const t = getActiveTab();
    if (t) t.view.webContents.goForward();
});
ipcMain.handle("reload", () => {
    const t = getActiveTab();
    if (t) t.view.webContents.reload();
});
ipcMain.handle("toggle-devtools", () => {
    const t = getActiveTab();
    if (t) t.view.webContents.toggleDevTools();
});

async function startTunnel() {
    return new Promise((resolve, reject) => {
        if (tunnelProc) {
            tunnelProc.kill();
            tunnelProc = null;
        }
        tunnelPort = null;

        const args = ["-transport", TUNNEL_TRANSPORT, "-proto", PROTO_MODE];
        if (AUTO_DISCOVER) {
            args.push("-discover=true");
            args.push("-discover-timeout", String(DISCOVER_TIMEOUT));
        } else {
            // 保留这个分支，万一以后需要手动指定远端时用
            args.push("-server", "192.168.1.100:8443");
        }

        console.log("tunnel args:", args);

        tunnelProc = spawn(path.join(__dirname, "proxy.client.exe"), args, {
            stdio: ["ignore", "pipe", "pipe"],
        });
        let gotPort = false;
        tunnelProc.stdout.on("data", (buf) => {
            if (gotPort) return;
            const str = buf.toString().trim();
            const p = parseInt(str, 10);
            if (!isNaN(p) && p > 1000 && p < 65535) {
                gotPort = true;
                tunnelPort = p;
                resolve(p);
            }
        });
        tunnelProc.stderr.on("data", (buf) =>
            console.log(`[Tunnel Log] ${buf.toString()}`)
        );
        tunnelProc.on("close", () => {
            tunnelProc = null;
            tunnelPort = null;
            clearProxy();
            mainWindow.webContents.send("tunnel-status", {
                ok: false,
                port: null,
            });
        });
        tunnelProc.on("error", reject);
    });
}
async function setProxy(port) {
    const ses = session.defaultSession;
    const proxyRule = `socks5://127.0.0.1:${port}`;
    await ses.setProxy({
        proxyRules: proxyRule,
        proxyBypassRules: "<local>;127.0.0.1;192.168.0.0/16;10.0.0.0/8",
    });
    ses.setHostResolverRules({
        rules: "MAP * ~NOTFOUND , EXCLUDE localhost, EXCLUDE 127.0.0.1",
    });
    mainWindow.webContents.send("tunnel-status", { ok: true, port });
}
async function clearProxy() {
    const ses = session.defaultSession;
    await ses.setProxy({ proxyRules: "" });
    ses.setHostResolverRules({ rules: "" });
}

app.on("ready", () => {
    const { globalShortcut } = require("electron");
    globalShortcut.register("Ctrl+T", () => createTab());
    globalShortcut.register("Ctrl+W", () => {
        const active = tabs.find((t) => t.id === activeTabId);
        if (active) closeTab(active.id);
    });
    globalShortcut.register("F5", () => {
        const t = getActiveTab();
        if (t) t.view.webContents.reload();
    });
    globalShortcut.register("F11", () => {
        if (mainWindow) mainWindow.setFullScreen(!mainWindow.isFullScreen());
    });
    globalShortcut.register("F12", () => {
        const t = getActiveTab();
        if (t) t.view.webContents.toggleDevTools();
    });
});
app.on("will-quit", () => require("electron").globalShortcut.unregisterAll());

app.whenReady().then(async () => {
    createWindow();
    try {
        const port = await startTunnel();
        await setProxy(port);
        createTab("https://www.baidu.com");
    } catch (e) {
        console.error("隧道启动失败", e);
    }
});
app.on("window-all-closed", () => {
    if (tunnelProc) tunnelProc.kill();
    app.quit();
});
