package weather

import (
	"fmt"
	"strings"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

// Profile- and level-specific guidance for the environmental ticker.
//
// Every profile has one message per AQI level (1 Good … 5 Very Poor), so the
// ticker reads differently for a runner, a parent doing the school run, a
// person with sensitive airways, someone deciding whether to open the windows,
// or an everyday user. Messages are localized under
// "pollution.ticker.msg.<profile>.<level>"; the English text below is the
// fallback used when no locale manager (or translation) is available.
//
// The planner is shared by the macOS, GTK and Fyne front ends, so all of them
// get identical messages.

// tickerGuidanceFallback[profile][level-1] is the English guidance text.
var tickerGuidanceFallback = map[config.TickerProfile][5]string{
	config.TickerProfileEveryday: {
		"Great day to be outdoors, enjoy it! 🌿",
		"Air is decent, no need to change your plans 🙂",
		"Air is so-so today, fine for a stroll but ease off longer stints outside 🚶",
		"Air is poor, cut back on long stints outdoors and take it easy 😷",
		"Air is very poor, stay indoors where you can and avoid outdoor exertion 🏠",
	},
	config.TickerProfileActive: {
		"Perfect air for a run or a ride, go get those miles in 🏃",
		"Air is fine for training, just listen to your body 🚴",
		"Air is moderate, shorten the hard efforts or swap to an easier session 🏃",
		"Air is poor, move your workout indoors today 🏋️",
		"Air is very poor, skip outdoor training and rest or train inside 🛑",
	},
	config.TickerProfileFamily: {
		"Lovely air for the school run and playground time 🧒",
		"Air is fine for the school run and play, nothing to worry about 🙂",
		"Air is moderate, keep outdoor play shorter and avoid busy roads 🚸",
		"Air is poor, keep the kids indoors for play and avoid busy roads 🏠",
		"Air is very poor, keep children inside and skip outdoor activities today 🛑",
	},
	config.TickerProfileSensitive: {
		"Clean air today, a comfortable day to be outside 🌿",
		"Air is fair, take extra care outdoors and keep things gentle 😷",
		"Air is moderate, limit time outside and take extra care 😷",
		"Air is poor, stay indoors where you can and avoid exertion 🏠",
		"Air is very poor, stay inside with the windows shut and avoid going out 🛑",
	},
	config.TickerProfileVentilation: {
		"Good time to open the windows and air the house out 🌬️",
		"Air is fine, a short airing with the windows open is OK 🌬️",
		"Air is moderate, keep windows closed during busy traffic hours 🚪",
		"Air is poor, keep the windows closed for now 🚪",
		"Air is very poor, keep windows and doors shut and skip airing out 🛑",
	},
}

// pollutantTipKey maps each metric to the suffix of its
// "pollution.ticker.tip.<suffix>" key, with an English fallback.
var pollutantTip = map[PollutionMetric]struct{ key, fallback string }{
	MetricAQI:  {"aqi", "A mix of pollutants, so check the individual readings 📊"},
	MetricCO:   {"co", "Mostly exhaust and gas heaters, so avoid enclosed spaces with engines running 🚗"},
	MetricNO:   {"no", "Mostly engine exhaust, so steer clear of heavy traffic 🚗"},
	MetricNO2:  {"no2", "Mostly traffic fumes, so avoid busy roads 🚗"},
	MetricO3:   {"o3", "Builds up on sunny afternoons, so head out early or later in the evening 🌇"},
	MetricSO2:  {"so2", "Mostly from industry and burning fuel, so keep away from smoky areas 🏭"},
	MetricNH3:  {"nh3", "Mostly from farming and fertiliser, so expect a whiff in rural areas 🌾"},
	MetricPM25: {"pm25", "Fine particles that reach deep into the lungs, so pick quieter routes 💨"},
	MetricPM10: {"pm10", "Mostly dust and dirt, so avoid dusty or building areas 🏗️"},
}

// tickerGuidance returns the localized message for a profile at an AQI level
// (clamped to 1-5).
func tickerGuidance(profile config.TickerProfile, level int, lm *i18n.LocaleManager) string {
	profile = config.NormalizeTickerProfile(profile)
	if level < 1 {
		level = 1
	} else if level > 5 {
		level = 5
	}
	key := fmt.Sprintf("pollution.ticker.msg.%s.%d", profile, level)
	return tr(lm, key, tickerGuidanceFallback[profile][level-1])
}

// pollutantTipText returns the localized pollutant-specific tip ("Mostly
// traffic fumes, so avoid busy roads"), or "" for an unknown metric.
func pollutantTipText(m PollutionMetric, lm *i18n.LocaleManager) string {
	t, ok := pollutantTip[m]
	if !ok {
		return ""
	}
	return tr(lm, "pollution.ticker.tip."+t.key, t.fallback)
}

// StripEmoji removes pictographic emoji (and their joiners/variation
// selectors) from s and collapses the leftover whitespace. Front ends whose
// text renderer cannot draw colour emoji (e.g. Fyne canvas text on Windows
// and Linux) use it so messages never show missing-glyph boxes. Symbols such
// as "⚠", "✓" and "ℹ" are kept.
func StripEmoji(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0x1F000 && r <= 0x1FAFF,
			r == 0xFE0F, r == 0x200D,
			r == 0x2600, r == 0x2614, r == 0x26C8, r == 0x2744:
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
