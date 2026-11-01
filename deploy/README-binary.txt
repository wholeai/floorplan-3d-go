户型装修设计 —— 单文件程序，无需安装 Go、无需安装 SQLite

启动
  ./floorplan3d                  浏览器打开 http://localhost:8787
  floorplan3d.exe                （Windows）双击即可

常用参数
  -addr :9000                    换端口
  -db ~/plan.db                  换数据库文件，默认 data/plan.db
  -base /floorplan               挂在子路径下（配合 nginx 反向代理）
  -version                       打印版本号

数据
  方案与用料清单都存在 -db 指向的那个 SQLite 文件里（默认 ./data/plan.db）。
  备份或换机器：连 data/ 目录一起拷走即可。

移除
  删掉本程序与 data/ 目录即可，不写注册表、不写系统目录。
