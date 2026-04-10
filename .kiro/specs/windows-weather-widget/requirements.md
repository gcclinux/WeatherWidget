# Requirements Document

## Introduction

A lightweight, always-on-top Windows 11 desktop weather widget built as a single native Go binary. The widget displays current weather conditions (icon, temperature, description), city name, and local date/time for one to three cities arranged side by side in a compact overlay pinned to a screen corner. Users configure the widget through a settings page where they manage their city list and choose between a remote weather API (such as OpenWeatherMap or Weather Underground) or a local PostgreSQL database as the data source.

## Glossary

- **Widget**: The always-on-top, borderless overlay window that displays weather information on the Windows 11 desktop.
- **City_Panel**: A single vertical column within the Widget that shows weather data for one configured city.
- **City_List**: The ordered collection of one to three cities configured by the user, each rendered as a City_Panel.
- **Settings_Page**: The configuration dialog that allows the user to manage the City_List, select a data source, and enter connection details.
- **Data_Source**: The provider of weather data — either a Remote_API or a Local_Database.
- **Remote_API**: An external HTTP-based weather service (e.g., OpenWeatherMap, Weather Underground) that returns weather data over the internet.
- **Local_Database**: A PostgreSQL database running on the local network or machine that stores weather, temperature, time, and location data.
- **Weather_Display**: The visual area within a City_Panel that renders the weather icon, temperature, description, city name, and date/time for that city.
- **API_Configuration_Form**: The section of the Settings_Page where the user enters Remote_API connection details such as the API key, endpoint URL, and city or coordinates.
- **Database_Configuration_Form**: The section of the Settings_Page where the user enters Local_Database connection details such as host, port, database name, username, and password.
- **Weather_Icon**: A graphical representation of the current weather condition (e.g., sunny, cloudy, rainy) displayed inside a City_Panel.

## Requirements

### Requirement 1: Widget Window Display

**User Story:** As a user, I want a compact overlay window pinned to a corner of my screen showing up to three cities side by side, so that I can compare weather across locations at a glance without it interfering with other applications.

#### Acceptance Criteria

1. WHEN the application starts, THE Widget SHALL render as a borderless, always-on-top window positioned in a corner of the primary display.
2. THE Widget SHALL remain visible above all other windows while the application is running.
3. THE Widget SHALL display one City_Panel for each city in the City_List, arranged horizontally side by side from left to right.
4. THE Widget SHALL support a minimum of 1 and a maximum of 3 cities in the City_List.
5. THE Widget SHALL dynamically resize its width to accommodate the number of configured City_Panel instances, with each City_Panel occupying no more than 300 × 120 device-independent pixels.
6. WHEN the user right-clicks the Widget, THE Widget SHALL display a context menu with options to open the Settings_Page, change the screen corner position, and exit the application.

### Requirement 2: Weather Data Rendering

**User Story:** As a user, I want to see the current weather conditions for each configured city displayed clearly in its own panel, so that I can quickly understand the weather across multiple locations.

#### Acceptance Criteria

1. EACH City_Panel SHALL show a Weather_Icon that visually represents the current weather condition for that city (e.g., sunny, cloudy, rainy, snowy, partly cloudy).
2. EACH City_Panel SHALL show the current temperature formatted as an integer followed by "°C" (e.g., "24°C").
3. EACH City_Panel SHALL show a short textual weather description (e.g., "Partial Sunny", "Heavy Rain").
4. EACH City_Panel SHALL show the configured city name and region abbreviation (e.g., "Holambra, SP").
5. EACH City_Panel SHALL show the current date and time in that city's local timezone formatted as "DD/MM/YYYY - HH:MM:SS" and update the time every second.

### Requirement 3: Data Source Selection

**User Story:** As a user, I want to choose between a remote weather API and a local PostgreSQL database, so that I can use whichever data source is available in my environment.

#### Acceptance Criteria

1. WHEN the user opens the Settings_Page, THE Settings_Page SHALL present a choice between "Remote API" and "Local Database" as the active Data_Source.
2. WHEN the user selects "Remote API", THE Settings_Page SHALL display the API_Configuration_Form.
3. WHEN the user selects "Local Database", THE Settings_Page SHALL display the Database_Configuration_Form.
4. THE Settings_Page SHALL persist the selected Data_Source and its configuration to a local configuration file so that the selection survives application restarts.
5. WHEN the user saves a new Data_Source selection, THE Widget SHALL immediately begin fetching weather data from the newly selected Data_Source for all cities in the City_List.

### Requirement 3a: City List Management

**User Story:** As a user, I want to add, remove, and reorder cities in my widget, so that I can monitor weather for the locations that matter to me.

#### Acceptance Criteria

1. THE Settings_Page SHALL display the current City_List with each city's name, region, and its position (1, 2, or 3).
2. WHEN the user adds a city, THE Settings_Page SHALL allow the user to enter a city name and region abbreviation (or coordinates) and append the city to the City_List.
3. IF the user attempts to add a city when the City_List already contains 3 cities, THEN THE Settings_Page SHALL display a message stating the maximum of 3 cities has been reached.
4. WHEN the user removes a city from the City_List, THE Widget SHALL remove the corresponding City_Panel and resize the Widget width accordingly.
5. WHEN the user reorders cities in the City_List, THE Widget SHALL rearrange the City_Panel positions to match the new order.
6. THE City_List SHALL contain at least 1 city at all times; THE Settings_Page SHALL prevent removal of the last remaining city.

### Requirement 4: Remote API Configuration

**User Story:** As a user, I want to configure a remote weather API with my own credentials, so that the widget can fetch live weather data from the internet.

#### Acceptance Criteria

1. THE API_Configuration_Form SHALL provide input fields for: API provider selection (OpenWeatherMap or Weather Underground), API key, and for each city in the City_List a city name or geographic coordinates (latitude and longitude).
2. WHEN the user submits the API_Configuration_Form with all required fields populated, THE Settings_Page SHALL validate that the API key is a non-empty string and that each city entry has a non-empty city name or valid coordinates, then save the configuration.
3. IF the user submits the API_Configuration_Form with one or more empty required fields, THEN THE Settings_Page SHALL highlight the empty fields and display a descriptive validation error message.
4. WHEN the user saves a valid Remote_API configuration, THE Widget SHALL attempt a test request to the configured API endpoint for the first city in the City_List and display a success or failure notification within the Settings_Page.

### Requirement 5: Local Database Configuration

**User Story:** As a user, I want to configure a local PostgreSQL connection, so that the widget can read weather data from my own database.

#### Acceptance Criteria

1. THE Database_Configuration_Form SHALL provide input fields for: host, port, database name, username, password, and the table/query used to retrieve weather data.
2. WHEN the user submits the Database_Configuration_Form with all required fields populated, THE Settings_Page SHALL validate that host, port, database name, and username are non-empty and that port is a valid integer between 1 and 65535.
3. IF the user submits the Database_Configuration_Form with invalid or missing required fields, THEN THE Settings_Page SHALL highlight the invalid fields and display a descriptive validation error message.
4. WHEN the user saves a valid Local_Database configuration, THE Widget SHALL attempt a test connection to the configured PostgreSQL instance and display a success or failure notification within the Settings_Page.

### Requirement 6: Weather Data Refresh

**User Story:** As a user, I want the weather data to update automatically at regular intervals, so that the displayed information stays current without manual intervention.

#### Acceptance Criteria

1. THE Widget SHALL fetch updated weather data from the active Data_Source for all cities in the City_List at a configurable interval with a default of 10 minutes.
2. WHEN the Settings_Page is open, THE Settings_Page SHALL allow the user to set the refresh interval to a value between 1 minute and 60 minutes.
3. IF a data fetch for a specific city fails, THEN THE corresponding City_Panel SHALL display the most recently successful weather data for that city and show a small error indicator icon.
4. IF three consecutive data fetches for a specific city fail, THEN THE corresponding City_Panel SHALL display a persistent warning message indicating the data for that city may be stale.

### Requirement 7: Single Native Binary and Minimal Size

**User Story:** As a user, I want the application delivered as a single executable with no external runtime dependencies, so that installation is simple and disk usage is minimal.

#### Acceptance Criteria

1. THE application SHALL compile to a single native Windows executable (.exe) using the Go toolchain with no external runtime or interpreter dependencies.
2. THE application SHALL embed all required assets (icons, fonts, images) within the compiled binary.
3. THE compiled binary SHALL target Windows 11 (amd64) as the primary platform.

### Requirement 8: Configuration Persistence

**User Story:** As a user, I want my settings to be saved automatically, so that I do not have to reconfigure the widget every time I start it.

#### Acceptance Criteria

1. WHEN the user saves settings through the Settings_Page, THE application SHALL write the configuration to a JSON file in the user's application data directory (e.g., %APPDATA%).
2. WHEN the application starts and a configuration file exists, THE application SHALL load the saved configuration and apply it without requiring user interaction.
3. IF the configuration file is missing or corrupted on startup, THEN THE application SHALL launch with default settings and open the Settings_Page automatically.

### Requirement 9: Application Lifecycle

**User Story:** As a user, I want the widget to start and stop cleanly, so that it does not leave orphan processes or consume resources after I close it.

#### Acceptance Criteria

1. WHEN the user selects "Exit" from the context menu, THE application SHALL stop all background data-fetch timers, release the Widget window, and terminate the process within 2 seconds.
2. WHEN the application starts and another instance is already running, THE application SHALL bring the existing instance to the foreground and terminate the new instance.
3. THE application SHALL add an icon to the Windows system tray that allows the user to show, hide, or exit the Widget.
