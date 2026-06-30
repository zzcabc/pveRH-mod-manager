# pveRH-mod-manager

植物大战僵尸融合版 Mod 管理器 — 本地 Web GUI 工具，管理 BepInEx 框架、植物/僵尸 MOD 和修改器。

## 快速开始

```powershell
# 构建
go build -o pveRH-mod-manager.exe .

# 运行（gamefile.json 需在同目录）
.\pveRH-mod-manager.exe

# 浏览器自动打开 http://127.0.0.1:19527
```

精简体积：
```powershell
go build -ldflags="-s -w" -o pveRH-mod-manager.exe .
```

## 功能

| 功能 | 说明 |
|---|---|
| BepInEx 管理 | 检测安装状态、一键安装/卸载 |
| 本地 MOD | 扫描 MOD 目录，按植物/僵尸分类，支持安装/卸载 |
| 在线 MOD | 对接服务器 API 浏览和下载远程 MOD |
| 修改器 | 匹配版本号，安装高数修改器 |
| 目录选择 | 调用系统原生文件夹选择器 |

## 项目结构

```
├── main.go              # 入口
├── app.go               # HTTP 服务器 + API
├── config.go            # 配置读写 + 版本检测
├── bepinex.go           # BepInEx 检测/安装/卸载
├── mod.go               # 本地 MOD 扫描/安装/卸载
├── modifier.go          # 修改器扫描/安装
├── online.go            # 服务器 API 客户端
├── utils.go             # 工具函数（解压、复制、文件夹选择器）
├── gamefile.json        # BepInEx 文件清单（随程序分发）
├── frontend/            # Web 前端（嵌入 exe）
│   ├── index.html
│   ├── main.js
│   └── style.css
└── docs/
    ├── 项目介绍.md
    └── 设计理解.md
```

## API 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/` | Web 前端 |
| GET/POST | `/api/config` | 读写配置 |
| GET | `/api/versions` | 版本列表 |
| GET | `/api/bepinex/check?path=` | BepInEx 状态 |
| GET | `/api/bepinex/install?path=` | 安装 BepInEx |
| GET | `/api/bepinex/uninstall?path=` | 卸载 BepInEx |
| GET | `/api/mods/local?modPath=&version=` | 本地 MOD 列表 |
| GET | `/api/mods/installed?path=` | 已安装 MOD |
| POST | `/api/mods/install` | 安装 MOD |
| POST | `/api/mods/uninstall` | 卸载 MOD |
| GET | `/api/mods/uninstall-all?path=` | 全部卸载 |
| GET | `/api/modifier/find?modPath=&version=` | 查找修改器 |
| POST | `/api/modifier/install` | 安装修改器 |
| GET | `/api/online/versions` | 在线版本 |
| GET | `/api/online/authors` | 在线作者 |
| GET | `/api/online/mods` | 在线 MOD |
| POST | `/api/online/install` | 下载安装在线 MOD |
| GET | `/api/select-folder` | 系统文件夹选择器 |
| GET | `/api/open-dir?path=` | 打开目录 |

## 配置文件

首次运行自动生成 `config.json`：

```json
{
  "game_path": [
    {"path": "D:\\Game\\植物大战僵尸融合版3.7Mod", "version": "3.7"}
  ],
  "mod_path": ["D:\\Game\\PVERH-MOD"],
  "download_path": "",
  "server_url": "https://pvzrh.zhaocheng.cc:8443"
}
```

## 依赖

- Go 1.25+
- Windows（文件夹选择器依赖 PowerShell）
- `gamefile.json`（与 exe 同目录）
