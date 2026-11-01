# 户型装修设计

中文 | [English](README.en.md)

户型装修设计工具：在 2D 平面图上摆放家具、拆改墙体、测量尺寸，一键切换到 Three.js 3D 场景，可以鸟瞰，也可以第一人称漫游。前端仍是单个 `index.html`（无构建步骤），配一个 Go + SQLite 后端保存方案与用料清单；也可以直接双击 `index.html` 用浏览器本地存储跑。

> **本项目基于 [wy51ai/floorplan-3d](https://github.com/wy51ai/floorplan-3d)（MIT，3.3k stars）改造**：
> 换成 Go + SQLite 后端、把 three.js 全部本地化、支持子目录部署，并新增装修风格套装与用料清单。
> 原项目的纯前端版本请直接访问上游仓库。

## 功能

**2D 平面布置**
- 按原始户型 1:60 / 1:100 比例显示，尺寸单位 mm
- 从左侧家具库拖入 60 余种家具家电（卧室、客厅、餐厨、卫浴、家电、书房休闲）
- 拖动移动、旋转（Shift 自由角度）、调整尺寸，贴墙自动吸附
- 测量工具（靠近墙面自动吸附，Shift 锁定水平 / 垂直）
- 拆改非承重墙，承重墙单独标示
- 图层开关：尺寸标注、房间名、家具、网格、承重墙

**3D 场景**
- 鸟瞰、斜视、俯视多种视角，点击房间列表可飞到对应房间
- 漫游模式：桌面端 WASD + 鼠标，触屏设备用虚拟摇杆，可以点门开关
- 全高墙 / 剖切墙切换，日照时间滑块，夜景灯光
- 精细家具模型：柜门分缝与拉手、软包床头、带环境反射的金属与陶瓷材质等
- 在 3D 中也能选中、拖动家具，与 2D 方案实时同步

**装修风格套装**
- 顶栏「风格」一键整套切换：现代简约 / 新中式 / 地中海 / 北欧
- 一套 = 各房间地面材质 + 3D 墙面漆色、墙顶饰面色 + 家具色板，一次 `mutate` 即一步撤销
- 「空房」开关：2D / 3D 同时隐藏家具，只看硬装毛坯

**用料与备注清单**
- 顶栏「用料清单」打开表格：分类 / 名称 / 品牌 / 规格型号 / 用量 / 单位 / 单价 / 备注
- 「从方案生成用量」自动带出地面（按材质汇总含 5% 损耗）、内墙乳胶漆（按墙长 × 层高扣门窗）、踢脚线（按周长）、家具 / 家电 / 柜体（按名称计数并写明所在房间）
- 品牌、规格、备注、单价自己填，重新生成时这些字段会保留；可导出 CSV（带 BOM，Excel 直接打开）

**方案与统计**
- 房间面积与套内使用面积自动统计
- 为每个房间更换地面材料（木地板、地砖、大理石、水磨石、地毯等），按面积加 5% 损耗估算造价
- 撤销 / 重做；方案与用料清单自动存进 SQLite（`data/plan.db`），用 `file://` 打开时退回浏览器本地存储
- 中文 / English 界面切换（顶栏右侧按钮，默认中文，选择会记住）
- 导出 PNG 图片，导出 / 导入方案 JSON

## 快速开始

```bash
git clone https://github.com/wholeai/floorplan-3d-go.git
cd floorplan-3d-go
go run .                                # http://localhost:8787，数据库 data/plan.db
go run . -addr :9000 -db ~/myplan.db    # 换端口 / 换库
go run . -base /floorplan               # 挂在子路径下：http://localhost:8787/floorplan/
```

打开 http://localhost:8787 即可。方案和用料清单写进 SQLite（纯 Go 驱动 [glebarez/go-sqlite](https://github.com/glebarez/go-sqlite)，不需要 cgo，也不需要装任何 SQLite 库）。

不想跑后端也行：直接用浏览器打开 `index.html`（或用 `python3 -m http.server 8000`），此时不连 `/api`，退化成浏览器本地存储。

```bash
go build -o floorplan3d .     # 单文件二进制（index.html 与 static/ 全部内嵌进二进制）
```

> **完全离线可用**：three.js r160 和用到的 addons 已经本地化到 `static/three/`，不请求任何 CDN。首次打开 3D 场景也不再需要联网。

### 部署到子目录（`https://www.test.com/floorplan`）

前端所有资源引用和接口地址都是相对路径，两种方式都支持：

**A. Go 自己带前缀**

```bash
go run . -base /floorplan        # 访问 /floorplan/ ，/floorplan 会 301 到 /floorplan/
```

**B. 反向代理剥前缀**（Nginx）

```nginx
location = /floorplan { return 301 /floorplan/; }   # 少了这行，相对路径会以根目录为基准
location /floorplan/ {
    proxy_pass http://127.0.0.1:8801/;      # 结尾的 / 会剥掉 /floorplan 前缀
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

完整的站点配置（含静态资源长缓存、PUT 请求体上限）见 [`deploy/nginx-floorplan.conf`](deploy/nginx-floorplan.conf)，
后端 `./floorplan3d -addr 127.0.0.1:8801` 起在回环地址上，只让 nginx 访问。

> 关键是**地址必须以 `/` 结尾**（`/floorplan/`）。`/floorplan` 后面没有斜杠时，浏览器会把相对路径解析到站点根目录，静态资源和 `/api` 全部 404，所以上面两处都做了 301。

## 快捷键

| 按键 | 作用 |
| --- | --- |
| `T` | 切换 2D / 3D |
| `V` / `M` / `X` | 选择 / 测量 / 拆改墙体 |
| `R` / `Shift+R` | 选中家具顺时针 / 逆时针旋转 90° |
| `Delete` / `Backspace` | 删除选中家具 |
| `Ctrl/⌘ + D` | 复制选中家具 |
| `Ctrl/⌘ + Z`，`Ctrl/⌘ + Shift + Z` | 撤销，重做 |
| `F` | 适应窗口 |
| `+` / `-` | 放大 / 缩小 |
| `[` / `]` | 展开 / 收起左侧家具库、右侧面板 |
| `Shift + F` | 全屏 |
| `Esc` | 取消当前操作 |
| 漫游：`WASD` / 方向键，`Shift`，`E` | 移动，快走，开关门 |

## 技术栈

- 原生 HTML / CSS / JavaScript，无框架、无构建步骤
- 2D 平面图用 SVG 绘制
- 3D 场景用 [Three.js](https://threejs.org/) r160（OrbitControls、PointerLockControls、RoundedBoxGeometry、RoomEnvironment、CSS2DRenderer）
- 后端 Go 1.26 标准库 `net/http` + SQLite（驱动 [`github.com/glebarez/go-sqlite`](https://github.com/glebarez/go-sqlite)，纯 Go 实现，无 cgo、无需系统安装 SQLite）
- 前端依赖零外链：three.js r160 与 5 个 addons 放在 `static/three/`，由 `importmap` 指向本地，`go:embed` 打包进二进制

### 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` / `PUT` | `/api/plan` | 整个方案（房间地面、家具、拆墙、风格）的 JSON，单行 `plans` 表 |
| `GET` / `PUT` | `/api/items` | 用料清单，整表替换式保存到 `items` 表 |

### 表结构

```sql
plans(id TEXT PRIMARY KEY, name TEXT, data TEXT, updated_at TEXT)
items(id INTEGER PRIMARY KEY AUTOINCREMENT, category, name, brand, spec,
      unit, qty REAL, price REAL, note, sort)
```

`data/` 已在 `.gitignore` 里，换机器把整个目录拷走即可。

## 自定义户型

户型数据写在 `index.html` 里：

- `ROOMS`：房间多边形、名称、默认地面材料
- `WALLS` / `WINS`：墙体与窗洞
- `MATS`：地面材料名称与单价
- `LIB`：家具库（类型、名称、默认尺寸、颜色）
- `STYLES`：装修风格套装（地面材质映射、墙 / 顶漆色、家具色板）
- `buildFurniture()`：各类家具的 3D 模型

改这些数据就能换成自己的户型。

### 升级 three.js

换版本只改 `static/three/` 下 6 个文件（`three.module.js` + `addons/{controls,geometries,environments,renderers}` 里用到的那 5 个），`index.html` 顶部的 `importmap` 不用动。

## 致谢

- 上游项目：[wy51ai/floorplan-3d](https://github.com/wy51ai/floorplan-3d) —— 2D / 3D 户型装修的原始实现（MIT），本项目的 `index.html` 前端由其改造而来
- [three.js](https://threejs.org/) r160（MIT），已本地化在 `static/three/`
- [glebarez/go-sqlite](https://github.com/glebarez/go-sqlite) —— 纯 Go 的 SQLite 驱动

## 社交媒体

- 原作者 X（Twitter）：[@akokoi1](https://x.com/akokoi1)
- 本仓库：https://github.com/wholeai/floorplan-3d-go

## 许可协议

[MIT](LICENSE) —— 与上游一致，保留原始版权声明 (c) 2026 wuyi。
