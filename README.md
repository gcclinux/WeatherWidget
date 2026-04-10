# EasyWidge

A compact weather widget for your desktop.

## Compilation

### Windows
```powershell
    $env:PATH = "C:\msys64\ucrt64\bin;" + $env:PATH; $env:CGO_ENABLED = "1"; gcc --version | Select-Object -First 1
```

```powershell
    go build -ldflags="-H windowsgui -s -w" -o weatherwidget.exe ./cmd/weatherwidget/
    .\weatherwidget.exe
```

### Linux
1. **Install dependencies**:
   ```bash
   sudo apt-get update && sudo apt-get install -y libgl1-mesa-dev xorg-dev
   ```
2. **Build**:
   ```bash
   make build
   ```

> **Note**: The first build may take several minutes as it compiles graphical dependencies (CGO). My updated Makefile includes the `-v` flag so you can monitor progress.

## Config example

```json
{
  "dataSource": "remote_api",
  "cities": [
    {
      "name": "Holambra",
      "region": "BR",
      "latitude": -22.6332,
      "longitude": -47.0545,
      "timezone": "America/Sao_Paulo"
    },
    {
      "name": "Edinburgh",
      "region": "UK",
      "latitude": 55.95,
      "longitude": -3.19,
      "timezone": "Europe/London"
    },
    {
      "name": "Warsaw",
      "region": "PL",
      "latitude": 52.23,
      "longitude": 21.01,
      "timezone": "Europe/Warsaw"
    }
  ],
  "refreshInterval": 30,
  "cornerPosition": "top-right",
  "apiConfig": {
    "provider": "openweathermap",
    "apiKey": "YOUR_API_KEY"
  }
}
```

### Windows COnfig location

```powershell
Get-Content "$env:APPDATA\WeatherWidget\WeatherWidget\config.json"
or
type "$env:APPDATA\WeatherWidget\WeatherWidget\config.json"
```

```bash
cat $HOME/.config/WeatherWidget/WeatherWidget/config.json
or
more $HOME/.config/WeatherWidget/WeatherWidget/config.json
```