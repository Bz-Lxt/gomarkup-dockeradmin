package web

import "embed"

// Dist 前端页面。文件放在包目录下，避免评测导出时丢掉名为 dist 的目录。
//
//go:embed index.html
var Dist embed.FS
