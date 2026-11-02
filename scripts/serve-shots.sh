#!/bin/sh
# 本地截 3D 图的两条命令（无头 Chrome，不需要开浏览器窗口）
#   ./scripts/serve-shots.sh           起服务
#   then: chrome --headless=new --screenshot=out.png "http://127.0.0.1:8810/?still=1&view=3d&show=living"
set -e
go run . -addr 127.0.0.1:8810 -db "${TMPDIR:-/tmp}/floorplan-shots.db" -no-open
