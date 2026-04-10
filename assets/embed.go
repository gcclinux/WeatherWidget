package assets

import "embed"

// Icons embeds all weather condition icon PNG files from the icons directory.
//
//go:embed icons/*.png
var Icons embed.FS
