# WeatherWidget Data Architecture & Settings Tabs Proposal

This document outlines the proposed tab structure, field availability, and layout distribution based on the unified API endpoint:  
`https://weather-gateway-ricardo.web.app/api/v1/weather-news/key={key}/{city},{country}`

---

## 1. Visual Layout & Target Slots

Based on the desktop widget interface:
```
+----------------------------------------------------------------------------------------------------+
| [Pin] City Name, RG        [SCROLLING TICKER: Alerts, News, Tips, Pollutants]                     |
|                                                                                                    |
|    23:18:42                +-------------------+-------------------+-------------------+           |
|   07/11/2026               |     TILE 1        |     TILE 2        |     TILE 3        |           |
|                            |   Humidity 89%    | Wind 6.0 km/h NW  | Wind Gust 0.0km/h |           |
|    [Icon]    3°C           +-------------------+-------------------+-------------------+           |
|         [Feels Like -1°C]  |     TILE 4        |     TILE 5        |     TILE 6        |           |
|          Overcast Clouds   |  Dew Point 1.7°C  | Pressure 1015 hPa |   UV Index 0.0    |           |
|                            +-------------------+-------------------+-------------------+           |
|                                                                                                    |
|  [AQI 2]      [NO 0.0]      [NO2 2.2]      [O3 74.5]      [NH3 0.1]      [PM2.5 1.0]      [...]    |
|   (Fair)       ug/m3         ug/m3          ug/m3          ug/m3          ug/m3                    |
+----------------------------------------------------------------------------------------------------+
```

1. **Left Hero Section (Core)**: City name, local time, date, condition icon, main temperature, feels like, description.
2. **Right 6-Tile Grid (2x3)**: 6 primary metric tiles with icon, label, and formatted value.
3. **Bottom Row (Air Quality & Pollution)**: Horizontal row of pollutant clouds with levels/values.
4. **Top Header Ticker**: Scrolling informative ticker (weather alerts, news items, lifestyle profile guidance, chosen pollutants).

---

## 2. Proposed Settings TAB Structure

| Tab Name | Purpose / Responsibility | Target Widget Placement |
| :--- | :--- | :--- |
| **Locations** | City management (1–5 cities), coordinate lookup, auto-fill. | Base widget multi-city configuration |
| **Appearance** | Layout modes, transparency, font sizes, units (°C/°F, km/h/mph), monitor selection. | Global styling |
| **Data Settings (Live Weather)** | Core hero toggles (time, date, temp) + Selection of 6 tiles from Real-Time Weather. | Left Hero + Right 6-Tile Grid |
| **Forecast: Today** | Today's forecast metrics, summary, rain probabilities, extremes. | Right 6-Tile Grid (or Forecast Mode) |
| **Forecast: Tomorrow** | Tomorrow's forecast outlook, temperatures, rain chance, winds. | Right 6-Tile Grid (or Forecast Mode) |
| **Air Quality** | Toggles for bottom card row (AQI + 8 chemical pollutants). | Bottom Card Row |
| **Ticker Settings** | Severe Alerts, News Items, Lifestyle Profiles, and Ticker-specific metrics (up to 6 per city). | Top Scrolling Ticker |
| **Data Provider** | API Key, Provider selection, refresh rate, connection test. | Backend gateway |
| **Language & About** | Locale selection (12 languages) & app version details. | Internationalization & System |

---

## 3. Data Dictionary by TAB

### TAB A: Data Settings (Real-Time Weather)
These metrics represent current conditions at the weather station.

| Field Name | API Key / JSON Path | Example (`Holambra,BR`) | Role / Destination |
| :--- | :--- | :--- | :--- |
| **City Name** | `city_name`, `country_code` | `"Holambra", "BR"` | Left Hero Title |
| **Local Time & Date** | `obstimelocal` | `"07/11/2026 17:31:51"` | Left Hero Time/Date |
| **Condition Icon** | `weather.FreeText` / WMO | `"heavy intensity rain"` | Left Hero Icon |
| **Temperature** | `weather.Temp` | `31.72 °C` | Left Hero Main Temp |
| **Feels Like** | `weather.feels_like` | `33.41 °C` | Left Hero Pill |
| **Weather Description** | `weather.description` | `"Heavy Intensity Rain"` | Left Hero Subtitle |
| **Humidity** | `weather.Humidity` | `48 %` | **6-Tile Grid Candidate** |
| **Wind Speed & Dir** | `weather.wind_speed`, `weather.wind_dir` | `3.12 m/s @ 58° (ENE)` | **6-Tile Grid Candidate** |
| **Wind Gust** | `weather.wind_gust` | `3.36 m/s` | **6-Tile Grid Candidate** |
| **Dew Point** | `weather.DewPt` | `19.36 °C` | **6-Tile Grid Candidate** |
| **Barometric Pressure**| `weather.Pressure` | `1007 hPa` | **6-Tile Grid Candidate** |
| **UV Index (Current)** | `weather.UV` | `0.54` | **6-Tile Grid Candidate** |
| **Sunrise Time** | `weather.sunrise` | `1794039608` (Unix seconds; `null` if unavailable) | **6-Tile Grid Candidate** |
| **Sunset Time** | `weather.sunset` | `1794086621` (Unix seconds; `null` if unavailable) | **6-Tile Grid Candidate** |
| **Cloud Coverage** | `weather.Clouds` | `56 %` | **6-Tile Grid Candidate** |
| **Visibility** | `weather.Visibility` | `10000 m` (10 km) | **6-Tile Grid Candidate** |
| **Precipitation Rate** | `weather.PrecipRate` | `2.4 mm/h` | **6-Tile Grid Candidate** |
| **Precipitation Total**| `weather.PrecipTotal` | `12.5 mm` | **6-Tile Grid Candidate** |
| **Solar Radiation** | `weather.SolarRadiation` | `W/m²` | **6-Tile Grid Candidate** |
| **Heat Index** | `weather.HeatIndex` | `33.5 °C` | **6-Tile Grid Candidate** |
| **Wind Chill** | `weather.WindChill` | `31.7 °C` | **6-Tile Grid Candidate** |

---

### Forecast Data — Today (Auto-selected via Grid Content dropdown)
Extracted from the `forecast` object for the current day. When "Today's Forecast" is selected in the Grid Content dropdown, all 6 numeric metrics below automatically populate the 6-tile grid:

| Field Name | API Key / JSON Path | Example (`Holambra,BR`) | Recommended Formatting / Icon |
| :--- | :--- | :--- | :--- |
| **Today Max Temp** | `forecast.today_temp_max` | `33.25 °C` | `🔺 High 33°C` |
| **Today Min Temp** | `forecast.today_temp_min` | `21.92 °C` | `🔻 Low 22°C` |
| **Rain Probability** | `forecast.today_precip_prob_max` | `100 %` | `☔ 100% Rain Chance` |
| **Expected Rain Amount**| `forecast.today_precip_sum_mm` | `19.99 mm` | `🌧️ 20.0 mm Expected` |
| **Peak UV Index** | `forecast.today_uv_max` | `14.51` (Extreme) | `☀️ Peak UV 14.5` |
| **Max Wind Gust** | `forecast.today_gust_max` | `11.71 m/s` (42 km/h) | `💨 Max Gust 42 km/h` |
| *(Ticker Feed)* **Today Synopsis** | `forecast.today_summary` | *"Expect a day of partly cloudy with rain"* | Ticker Headline (`news_items`) |

---

### Forecast Data — Tomorrow (Auto-selected via Grid Content dropdown)
Extracted from the `forecast` object for the upcoming day. When "Tomorrow's Forecast" is selected in the Grid Content dropdown, all 6 numeric metrics below automatically populate the 6-tile grid:

| Field Name | API Key / JSON Path | Example (`Holambra,BR`) | Recommended Formatting / Icon |
| :--- | :--- | :--- | :--- |
| **Tomorrow Max Temp** | `forecast.tomorrow_temp_max` | `23.90 °C` | `🔺 High 24°C` |
| **Tomorrow Min Temp** | `forecast.tomorrow_temp_min` | `16.68 °C` | `🔻 Low 17°C` |
| **Tomorrow Rain Chance**| `forecast.tomorrow_precip_prob_max`| `100 %` | `☔ 100% Rain Chance` |
| **Tomorrow Rain Amount**| `forecast.tomorrow_precip_sum_mm`| `1.07 mm` | `🌧️ 1.1 mm Expected` |
| **Tomorrow Peak UV** | `forecast.tomorrow_uv_max` | `2.16` (Low) | `☀️ Peak UV 2.2` |
| **Tomorrow Max Gust** | `forecast.tomorrow_gust_max` | `15.24 m/s` (55 km/h) | `💨 Max Gust 55 km/h` |
| *(Ticker Feed)* **Tomorrow Synopsis** | `forecast.tomorrow_summary` | *"Expect a day of partly cloudy with rain"* | Ticker Headline (`news_items`) |

---

### TAB D: Air Quality & Pollution (Bottom Row of Card)
Selected independently from the 6-tile grid. These display along the bottom row as colored cloud pills.

| Field Name | API Key / JSON Path | Example (`Holambra,BR`) | Display Type |
| :--- | :--- | :--- | :--- |
| **Air Quality Index** | `pollution.aqi` | `2 (Fair)` | Cloud icon + Category text |
| **Particulates PM2.5** | `pollution.pm2_5` | `19.65 µg/m³` | Cloud icon + µg/m³ |
| **Particulates PM10** | `pollution.pm10` | `19.93 µg/m³` | Cloud icon + µg/m³ |
| **Ozone (O₃)** | `pollution.o3` | `62.70 µg/m³` | Cloud icon + µg/m³ |
| **Nitrogen Dioxide (NO₂)** | `pollution.no2` | `1.98 µg/m³` | Cloud icon + µg/m³ |
| **Sulphur Dioxide (SO₂)** | `pollution.so2` | `0.38 µg/m³` | Cloud icon + µg/m³ |
| **Carbon Monoxide (CO)** | `pollution.co` | `307.27 µg/m³` | Cloud icon + µg/m³ |
| **Nitric Oxide (NO)** | `pollution.no` | `0.07 µg/m³` | Cloud icon + µg/m³ |
| **Ammonia (NH₃)** | `pollution.nh3` | `1.72 µg/m³` | Cloud icon + µg/m³ |

---

### TAB E: Ticker Settings (Top Scrolling Bar)
The ticker can cycle up to **6 rotating items per city**, prioritized by urgency. Users can enable or disable the ticker via **[ ] Enable Ticker**, toggle **Health Profiles** (multi-select: Everyday Outdoors, Runner & Cyclist, Family & School Run, Sensitive Airways, Open Windows — can select any or none), configure **Alerts & News Feed** headlines, and select individual **Air Quality & Pollution** metrics:

| Category / Toggle | Source in API | Sample Content | Behavior / Priority |
| :--- | :--- | :--- | :--- |
| **Emergency Alerts** | `alert` / `news_items[category=alert]` | `⚠️ Tempestade: Chuva entre 20 e 30 mm/h, ventos intensos (40-60 km/h) - INMET` | **Priority 1 (Red / Warning)**: Always shown first when active. |
| **Today's Outlook News** | `news_items[category=today_outlook]` | `Today's Outlook: High 33.2°C / Low 21.9°C • Currently 31.7°C (feels like 33.4°C)` | **Priority 2 (Informative)**: Daily outlook digest. |
| **Precipitation Watch** | `news_items[category=precipitation]` | `Precipitation Watch: 100% Chance of Rain • Up to 20.0 mm expected today` | **Priority 2 (Actionable)**: Warns if rain/snow is forecasted. |
| **Tomorrow's Forecast News** | `news_items[category=tomorrow_outlook]`| `Tomorrow's Forecast: Slight rain • 23.9°C / 16.7°C (100% rain prob)` | **Priority 3 (Forward Look)**: Preview for next day. |
| **Health Profiles** | Calculated from AQI + active profiles (Everyday, Active, Family, Sensitive, Ventilation) | `🏃 Active Profile: Fresh, clean air • Perfect for outdoor exercise` | **Priority 3 (Health Advice)**: Tailored advice per selected profile (user can choose any or none). |
| **Individual Pollutants**| `pollution.*` (AQI, PM2.5, O₃, etc.) | `Ozone (O₃): 62.7 µg/m³ (Good) • Builds up in afternoon sunlight` | **Priority 4 (Specific Metrics)**: Appended if checked by user. |
| **Sun & Weather Tips** | `weather.Sunset`, `weather.Sunrise`, Rain | `Sunset at 18:43 • Carry an umbrella ☔ and take a torch 🔦` | **Priority 4 (Micro-advice)**: Contextual day/night weather advice. |

---

## 4. How the 6-Tile Grid Can Work (Design Choices)

### Choice 1: "Grid Mode" Dropdown + Metric Selection
In Settings, the user picks the **Grid View Mode**:
* **Mode A: Live Conditions (Default)**: Shows 6 tiles from Real-Time Weather (e.g. Humidity, Wind, Gust, Dew Pt, Pressure, UV).
* **Mode B: Today's Forecast**: Shows 6 tiles from Today's Outlook (e.g. High/Low, Rain Chance %, Expected mm, Max UV, Max Gust, Today Summary).
* **Mode C: Tomorrow's Forecast**: Shows 6 tiles for Tomorrow (e.g. Tomorrow High/Low, Tomorrow Rain Chance %, Rain mm, Peak UV, Max Gust, Summary).
* **Mode D: Hybrid / Custom**: User checks any 6 checkboxes across Live + Today + Tomorrow, and they fill the 6 tiles in order.

### Choice 2: Free 6-Slot Pick List (Custom Layout)
In the UI, provide 6 dropdown selectors:
* Slot 1: `[ Humidity (Live) ▾ ]`
* Slot 2: `[ Wind Speed & Dir (Live) ▾ ]`
* Slot 3: `[ Today High/Low (Forecast) ▾ ]`
* Slot 4: `[ Rain Chance (Today) ▾ ]`
* Slot 5: `[ Pressure (Live) ▾ ]`
* Slot 6: `[ Sunset Time (Live) ▾ ]`
*(Allows users complete freedom to mix live observations with forecast outlooks).*

---

## 5. Summary of Next Steps for Implementation

1. **Review & Choose Preferred Model**: Review this table and decide if you prefer **Choice 1 (Dedicated Mode/Tabs)** or **Choice 2 (Custom 6-slot pick list)** for the 6-tile grid.
2. **Update Go Data Structures (`internal/weather`)**: Unmarshal `forecast`, `alert`, and `news_items` structs from the unified endpoint.
3. **Extend Config (`internal/config`)**: Add field definitions for the selected 6-tile layout and ticker items.
4. **Build the Settings Tabs**: Add the `Today`, `Tomorrow`, and enhanced `Ticker` sections into `ui-windows`.
5. **Render in Card & Ticker**: Update the Cairo/Windows graphics pipeline to draw the selected tiles and cycle the new ticker news items.
