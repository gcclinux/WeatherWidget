package assets

import "embed"

// Icons embeds all weather condition icon files (PNG, GIF, WebP) from the icons directory.
//
//go:embed icons/*
var Icons embed.FS

// DemoPNG embeds the demo screenshot shown in the About tab.
//
//go:embed demo.png
var DemoPNG []byte

// Fonts embeds custom TTF font files for crisp UI typography.
//
//go:embed fonts/*.ttf
var Fonts embed.FS
