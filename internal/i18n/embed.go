package i18n

import "embed"

// LocaleFS embeds all locale JSON files from the locales directory.
//
//go:embed locales/*.json
var LocaleFS embed.FS
