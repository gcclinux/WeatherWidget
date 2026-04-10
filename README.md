# EasyWidge

EasyWidge


`$env:PATH = "C:\msys64\ucrt64\bin;" + $env:PATH; $env:CGO_ENABLED = "1"; gcc --version | Select-Object -First 1`

```
go build -ldflags="-H windowsgui -s -w" -o weatherwidget.exe ./cmd/weatherwidget/
.\weatherwidget.exe
```