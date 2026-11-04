# Pollution Ticker Messages (`pollution.ticker.*`)

Reference for the en-GB texts and when each is triggered. Source of truth for logic:
[`planPollutionTicker`](../../weather/pollution.go) and [`ticker_guidance.go`](../../weather/ticker_guidance.go).

## How the ticker decides

1. The **dominant pollutant** is the one with the highest level (1–5) among the pollutants the user selected
   (first in `PollutionMetricOrder` wins ties). Levels: 1 Good, 2 Fair, 3 Moderate, 4 Poor, 5 Very Poor.
2. Each **profile** has an alert threshold: Everyday = 4, Runner & Cyclist = 3, Family = 3, Sensitive = 2, Open Windows = 3.
3. Result by dominant level `L`:
   - `L >= threshold` → **Alert** item (`⚠ Alert`), detail = profile message + pollutant tip.
   - else `L >= 3` → **Context** item first, detail = profile message + pollutant tip.
   - else (`L <= 2`) → **Opportunity** item (`✓ Opportunity`): headline is "Fresh, clean air" at level 1 or "Air quality is decent" at level 2;
     detail is the weather advice if any applies, otherwise the profile message.
4. The `.alt` variant of a message is chosen when `(fnv32(cityName) + localHour) % 2 == 1`; otherwise the base text.

## Profile messages — `pollution.ticker.msg.<profile>.<level>`

Triggered by the dominant level (clamped 1–5) for the user's chosen profile. Shown as the detail line of the Alert / Context
item, or of the Opportunity item when no weather advice applies.

| Key | Message | Profile | Level | Shown as |
|---|---|---|---|---|
| `msg.everyday.1` | Great day to be outdoors, enjoy it! 🌿 | Everyday | 1 Good | Opportunity (if no weather advice) |
| `msg.everyday.2` | Air is decent, no need to change your plans 🙂 | Everyday | 2 Fair | Opportunity (if no weather advice) |
| `msg.everyday.3` | Air is so-so today, fine for a stroll but ease off longer stints outside 🚶 | Everyday | 3 Moderate | Context (below alert threshold 4) |
| `msg.everyday.4` | Air is poor, cut back on long stints outdoors and take it easy 😷 | Everyday | 4 Poor | Alert |
| `msg.everyday.5` | Air is very poor, stay indoors where you can and avoid outdoor exertion 🏠 | Everyday | 5 Very Poor | Alert |
| `msg.active.1` | Perfect air for a run or a ride, go get those miles in 🏃 | Runner & Cyclist | 1 Good | Opportunity (if no weather advice) |
| `msg.active.2` | Air is fine for training, just listen to your body 🚴 | Runner & Cyclist | 2 Fair | Opportunity (if no weather advice) |
| `msg.active.3` | Air is moderate, shorten the hard efforts or swap to an easier session 🏃 | Runner & Cyclist | 3 Moderate | Alert |
| `msg.active.4` | Air is poor, move your workout indoors today 🏋️ | Runner & Cyclist | 4 Poor | Alert |
| `msg.active.5` | Air is very poor, skip outdoor training and rest or train inside 🛑 | Runner & Cyclist | 5 Very Poor | Alert |
| `msg.family.1` | Lovely air for the school run and playground time 🧒 | Family & School Run | 1 Good | Opportunity (if no weather advice) |
| `msg.family.2` | Air is fine for the school run and play, nothing to worry about 🙂 | Family & School Run | 2 Fair | Opportunity (if no weather advice) |
| `msg.family.3` | Air is moderate, keep outdoor play shorter and avoid busy roads 🚸 | Family & School Run | 3 Moderate | Alert |
| `msg.family.4` | Air is poor, keep the kids indoors for play and avoid busy roads 🏠 | Family & School Run | 4 Poor | Alert |
| `msg.family.5` | Air is very poor, keep children inside and skip outdoor activities today 🛑 | Family & School Run | 5 Very Poor | Alert |
| `msg.sensitive.1` | Clean air today, a comfortable day to be outside 🌿 | Sensitive Airways | 1 Good | Opportunity (if no weather advice) |
| `msg.sensitive.2` | Air is fair, take extra care outdoors and keep things gentle 😷 | Sensitive Airways | 2 Fair | Alert (threshold 2) |
| `msg.sensitive.3` | Air is moderate, limit time outside and take extra care 😷 | Sensitive Airways | 3 Moderate | Alert |
| `msg.sensitive.4` | Air is poor, stay indoors where you can and avoid exertion 🏠 | Sensitive Airways | 4 Poor | Alert |
| `msg.sensitive.5` | Air is very poor, stay inside with the windows shut and avoid going out 🛑 | Sensitive Airways | 5 Very Poor | Alert |
| `msg.ventilation.1` | Good time to open the windows and air the house out 🌬️ | Open Windows | 1 Good | Opportunity (if no weather advice) |
| `msg.ventilation.2` | Air is fine, a short airing with the windows open is OK 🌬️ | Open Windows | 2 Fair | Opportunity (if no weather advice) |
| `msg.ventilation.3` | Air is moderate, keep windows closed during busy traffic hours 🚪 | Open Windows | 3 Moderate | Alert |
| `msg.ventilation.4` | Air is poor, keep the windows closed for now 🚪 | Open Windows | 4 Poor | Alert |
| `msg.ventilation.5` | Air is very poor, keep windows and doors shut and skip airing out 🛑 | Open Windows | 5 Very Poor | Alert |

## Pollutant tips — `pollution.ticker.tip.<metric>`

Appended (` • tip`) to the profile message for Alert and Context items (levels where the dominant level is 3 or above, or
at/above the profile threshold). Not appended on Opportunity items. The metric is the dominant pollutant.

| Key | Message | Trigger (dominant pollutant) | Level |
|---|---|---|---|
| `tip.aqi` | A mix of pollutants, so check the individual readings 📊 | Overall AQI | ≥ 3, or ≥ profile threshold |
| `tip.co` | Mostly exhaust and gas heaters, so avoid enclosed spaces with engines running 🚗 | CO | ≥ 3, or ≥ profile threshold |
| `tip.no` | Mostly engine exhaust, so steer clear of heavy traffic 🚗 | NO | ≥ 3, or ≥ profile threshold |
| `tip.no2` | Mostly traffic fumes, so avoid busy roads 🚗 | NO₂ | ≥ 3, or ≥ profile threshold |
| `tip.o3` | Builds up on sunny afternoons, so head out early or later in the evening 🌇 | O₃ | ≥ 3, or ≥ profile threshold |
| `tip.so2` | Mostly from industry and burning fuel, so keep away from smoky areas 🏭 | SO₂ | ≥ 3, or ≥ profile threshold |
| `tip.nh3` | Mostly from farming and fertiliser, so expect a whiff in rural areas 🌾 | NH₃ | ≥ 3, or ≥ profile threshold |
| `tip.pm25` | Fine particles that reach deep into the lungs, so pick quieter routes 💨 | PM2.5 | ≥ 3, or ≥ profile threshold |
| `tip.pm10` | Mostly dust and dirt, so avoid dusty or building areas 🏗️ | PM10 | ≥ 3, or ≥ profile threshold |

(For the Sensitive profile the threshold is 2, so tips also appear at level 2.)

## Weather advice — `pollution.ticker.weather.*`

Only used on the **Opportunity** item (dominant level ≤ 2) when weather data is available; it replaces the profile message.
Checked in this order, the first match is the main clause. A night/UV clause may be appended with "and".

| Key | Message | Trigger |
|---|---|---|
| `weather.storm` | Thunderstorms about, best to stay indoors ⛈️ | Storm icon or description contains "thunder" |
| `weather.storm.alt` | Stormy out there, better to stay inside ⛈️ | Same, alt variant |
| `weather.snow` | It's snowing, so wrap up and watch for ice ❄️ | Snow icon, or description contains "snow"/"sleet" |
| `weather.snow.alt` | Snow about, so wrap up warm and mind the ice ❄️ | Same, alt variant |
| `weather.rain` | It's raining, so you may get wet, take an umbrella ☔ | Rain/heavy-rain icon, or description contains "rain"/"drizzle"/"shower" |
| `weather.rain.alt` | Rain about, so bring an umbrella to stay dry ☔ | Same, alt variant |
| `weather.fog` | It's foggy, so take care out there 🌫️ | Fog icon, or description contains "fog"/"mist" |
| `weather.fog.alt` | Visibility is low in the fog, so take it easy 🌫️ | Same, alt variant |
| `weather.cold` | Feels like %s, so wrap up warm 🧥 | Feels-like ≤ 5 °C (and none of the above) |
| `weather.cold.alt` | Brr, it feels like %s, so dress warm 🧥 | Same, alt variant |
| `weather.hot` | Feels like %s, so drink plenty of water and find some shade ☀️ | Feels-like ≥ 30 °C (and none of the above) |
| `weather.hot.alt` | It's hot out there, feels like %s, so stay hydrated and seek shade ☀️ | Same, alt variant |
| `weather.night` | It's dark out, so take a torch 🔦 | Local hour < 6 or ≥ 18, used alone when there is no main clause |
| `weather.night.add` | take a torch 🔦 | Night, appended to the main clause |
| `weather.uv` | The UV is high, so wear sun protection 😎 | Daytime and UV index ≥ 6, used alone when there is no main clause |
| `weather.uv.add` | wear sun protection 😎 | Daytime and UV ≥ 6, appended to the main clause |
| `weather.and` | and | Joins the main clause and the night/UV extra |

If no weather condition applies, the profile message (`msg.<profile>.<level>`) is shown instead.

## Headlines, labels and actions

| Key | Message | Trigger |
|---|---|---|
| `opportunity` | Opportunity | Label of the Opportunity item (dominant level ≤ 2 and below profile threshold) |
| `opportunity.message` | Fresh, clean air 🌿 | Opportunity headline at level 1 |
| `opportunity.message.alt` | Lovely air out there 🌿 | Same, alt variant |
| `opportunity.fair` | Air quality is decent 🙂 | Opportunity headline at level 2 (non-sensitive profiles) |
| `opportunity.fair.alt` | Pretty good air today 🙂 | Same, alt variant |
| `opportunity.now` | A great time to head outside | Not used by the app code (only referenced in locale tests) |
| `opportunity.now.alt` | Perfect for a walk or a run | Not used by the app code (only referenced in locale tests) |
| `alert` | Alert | Label of the Alert item (dominant level ≥ profile threshold) |
| `context` | Context | Label of the Context item (always added after the Alert/Opportunity item, and first at level 3 below threshold) |
| `currently` | Currently | Label on each pollutant context entry |
| `sources` | Primary sources | Label for each pollutant's sources text |
| `action.everyday` | consider reducing prolonged outdoor activity | Profile action text, Everyday (alert level 4) |
| `action.active` | reduce strenuous outdoor activity | Profile action text, Runner & Cyclist (alert level 3) |
| `action.family` | limit prolonged outdoor time for children | Profile action text, Family (alert level 3) |
| `action.sensitive` | take extra care outdoors | Profile action text, Sensitive (alert level 2) |
| `action.ventilation` | keep windows closed for now | Profile action text, Open Windows (alert level 3) |

`action.*` texts are resolved by `tickerProfileDetails` but the ticker planner currently only uses the alert level from it.
