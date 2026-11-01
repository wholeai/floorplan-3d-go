# Floor Plan Interior Designer

[中文](README.md) | English

An interior design tool for your own floor plan: place furniture, remove or modify walls, and take measurements on a 2D plan, then switch to a Three.js 3D scene with one click — view it from above or walk through it in first person. The front end is still a single `index.html` with no build step; a small Go + SQLite backend stores the plan and the materials list. You can also just open `index.html` and run it on browser storage alone.

> **Built on top of [wy51ai/floorplan-3d](https://github.com/wy51ai/floorplan-3d)** (MIT, 3.3k stars):
> this fork adds a Go + SQLite backend, vendors three.js locally, supports sub-path deployment,
> and adds style packs plus a materials/notes list. For the original pure front-end app,
> go straight to the upstream repository.

## Features

**2D Floor Plan**
- Displays the original floor plan at 1:60 / 1:100 scale, dimensions in mm
- Drag 60+ furniture and appliance items from the library on the left (bedroom, living room, dining & kitchen, bathroom, appliances, study & leisure)
- Move, rotate (hold Shift for free angle), and resize items, with automatic snapping to walls
- Measuring tool (snaps to nearby walls; hold Shift to lock horizontal / vertical)
- Remove or modify non-load-bearing walls; load-bearing walls are marked separately
- Layer toggles: dimensions, room names, furniture, grid, load-bearing walls

**3D Scene**
- Bird's-eye, oblique, and top-down views; click a room in the list to fly to it
- Walkthrough mode: WASD + mouse on desktop, virtual joystick on touch devices; click doors to open / close them
- Toggle between full-height and cut-away walls, time-of-day sunlight slider, night lighting
- Detailed furniture models: cabinet door gaps and handles, upholstered headboards, metal and ceramic materials with environment reflections, and more
- Select and drag furniture in 3D as well, kept in sync with the 2D plan in real time

**Style Packs**
- One dropdown in the top bar switches the whole scheme: Modern Minimal / New Chinese / Mediterranean / Nordic
- A pack = per-room flooring + 3D wall paint, trim and ceiling colour + furniture palette, applied as one undoable step
- "Shell" toggle hides every piece of furniture in 2D and 3D so you can look at the bare structure

**Materials & Notes**
- "BOM" button opens an editable table: category, item, brand, spec/model, qty, unit, unit price, notes
- "Fill from plan" derives quantities: flooring per material (incl. 5% waste), interior paint (wall length × ceiling height minus openings), skirting (room perimeters), and furniture / appliances / cabinets counted by name with the rooms they sit in
- Brand, spec, notes and price are yours to fill and survive regeneration; export to CSV (with BOM, opens directly in Excel)

**Plans & Statistics**
- Automatic calculation of room areas and net usable floor area
- Change the floor material of each room (wood, tiles, marble, terrazzo, carpet, etc.), with cost estimates based on area plus 5% wastage
- Undo / redo; the plan and materials list are saved to SQLite (`data/plan.db`), falling back to browser storage when opened as a `file://` page
- Chinese / English UI toggle (button on the right of the top bar; defaults to Chinese and remembers your choice)
- Export to PNG, export / import plans as JSON

## Quick Start

```bash
git clone https://github.com/wholeai/floorplan-3d-go.git
cd floorplan-3d-go
```

```bash
go run .                                 # http://localhost:8787, database at data/plan.db
go run . -addr :9000 -db ~/myplan.db     # different port / database
go run . -base /floorplan                # mount under a sub-path: http://localhost:8787/floorplan/
```

Open http://localhost:8787. Plans and the materials list go into SQLite (pure-Go driver [glebarez/go-sqlite](https://github.com/glebarez/go-sqlite) — no cgo, no system SQLite install).

No backend required either: open `index.html` directly (or `python3 -m http.server 8000`) and the app skips `/api` and keeps everything in browser storage.

```bash
go build -o floorplan3d .     # single binary, index.html and static/ are embedded
```

> **Fully offline**: three.js r160 and the addons it uses are vendored under `static/three/`. No CDN requests at all.

### Deploying under a sub-path (`https://www.test.com/floorplan`)

Every asset reference and API URL is relative, so both setups work:

**A. Let Go own the prefix**

```bash
go run . -base /floorplan        # serve at /floorplan/ ; /floorplan 301s to /floorplan/
```

**B. Strip the prefix in a reverse proxy** (Nginx)

```nginx
location = /floorplan { return 301 /floorplan/; }   # without this, relative URLs resolve against the site root
location /floorplan/ {
    proxy_pass http://127.0.0.1:8801/;      # trailing / strips the /floorplan prefix
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

A complete site config (long-lived caching for the vendored assets, PUT body limit) is in
[`deploy/nginx-floorplan.conf`](deploy/nginx-floorplan.conf). Run the backend on the loopback address
(`./floorplan3d -addr 127.0.0.1:8801`) so only nginx can reach it.

> The URL **must end with a slash** (`/floorplan/`). At `/floorplan` the browser resolves relative paths against the site root, so every asset and `/api` call 404s — hence the 301s above.

## Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `T` | Toggle 2D / 3D |
| `V` / `M` / `X` | Select / Measure / Modify walls |
| `R` / `Shift+R` | Rotate selected furniture 90° clockwise / counterclockwise |
| `Delete` / `Backspace` | Delete selected furniture |
| `Ctrl/⌘ + D` | Duplicate selected furniture |
| `Ctrl/⌘ + Z`, `Ctrl/⌘ + Shift + Z` | Undo, redo |
| `F` | Fit to window |
| `+` / `-` | Zoom in / out |
| `[` / `]` | Show / hide the furniture library (left) and the side panel (right) |
| `Shift + F` | Fullscreen |
| `Esc` | Cancel current action |
| Walkthrough: `WASD` / arrow keys, `Shift`, `E` | Move, walk faster, open / close doors |

## Tech Stack

- Vanilla HTML / CSS / JavaScript — no framework, no build step
- 2D floor plan rendered with SVG
- 3D scene built with [Three.js](https://threejs.org/) r160 (OrbitControls, PointerLockControls, RoundedBoxGeometry, RoomEnvironment, CSS2DRenderer)
- Backend: Go 1.26 `net/http` + SQLite (driver [`github.com/glebarez/go-sqlite`](https://github.com/glebarez/go-sqlite): pure-Go implementation, no cgo, no system SQLite needed)
- No external front-end dependencies: three.js r160 and its 5 addons live in `static/three/`, wired up by `importmap` and embedded with `go:embed`

### API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` / `PUT` | `/api/plan` | Whole plan JSON (room flooring, furniture, demolished walls, style) in one `plans` row |
| `GET` / `PUT` | `/api/items` | Materials list, saved by replacing the whole `items` table |

### Schema

```sql
plans(id TEXT PRIMARY KEY, name TEXT, data TEXT, updated_at TEXT)
items(id INTEGER PRIMARY KEY AUTOINCREMENT, category, name, brand, spec,
      unit, qty REAL, price REAL, note, sort)
```

`data/` is git-ignored; copy the directory to move your plan to another machine.

## Customizing the Floor Plan

The floor plan data lives in `index.html`:

- `ROOMS`: room polygons, names, and default floor materials
- `WALLS` / `WINS`: walls and window openings
- `MATS`: floor material names and unit prices
- `LIB`: furniture library (type, name, default size, color)
- `STYLES`: style packs (flooring map, wall/ceiling paint, furniture palette)
- `buildFurniture()`: 3D models for each furniture type

Edit this data to use your own floor plan.

### Upgrading three.js

Replace the six files under `static/three/` (`three.module.js` plus the five addons under `addons/{controls,geometries,environments,renderers}`); the `importmap` at the top of `index.html` stays as is.

## Credits

- Upstream: [wy51ai/floorplan-3d](https://github.com/wy51ai/floorplan-3d) — the original 2D/3D floor-plan designer (MIT); the `index.html` front end here is derived from it
- [three.js](https://threejs.org/) r160 (MIT), vendored under `static/three/`
- [glebarez/go-sqlite](https://github.com/glebarez/go-sqlite) — pure-Go SQLite driver

## Social Media

- Original author on X: [@akokoi1](https://x.com/akokoi1)
- This repo: https://github.com/wholeai/floorplan-3d-go

## License

[MIT](LICENSE) — same as upstream, original copyright notice (c) 2026 wuyi retained.
