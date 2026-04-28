package assets

import "embed"

// Icons embeds all weather condition icon PNG files from the icons directory.
//
//go:embed icons/*.png
var Icons embed.FS

// DemoPNG embeds the demo screenshot shown in the About tab.
//
//go:embed demo.png
var DemoPNG []byte
