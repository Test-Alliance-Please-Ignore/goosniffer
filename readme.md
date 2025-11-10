# GooSniffer 🌕👃
A simple Windows clipboard watcher that automatically detects EVE Online moon scan data, parses it into structured JSON, and optionally uploads it to an API endpoint.

---

## 🧩 Features

- Watches your **Windows clipboard** in real time
- Detects moon scan data (lines containing “Moon Product”)
- Parses scan text into structured JSON grouped by moon and product
- Optionally POSTs the JSON to a configured endpoint with a Bearer token

---

## 🚀 Usage

You can configure the upload target via **flags** or **environment variables** — both are optional.  
If no configuration is provided, GooSniffer just logs and prints the parsed JSON locally.

| Flag | Environment | Description |
|------|--------------|-------------|
| `--api-endpoint` | `API_ENDPOINT` | URL to POST parsed moon scan data to |
| `--api-token` | `API_TOKEN` | Bearer token for API authentication |

Example:

```powershell
# Windows PowerShell script example
setx API_ENDPOINT "https://example.com/api/moonscan"
setx API_TOKEN "your-secret-token"

goosniffer.exe
```

Or with flags:

```powershell
goosniffer.exe --api-endpoint "https://example.com/api/moonscan" --api-token "your-secret-token"
```

1. Copy a block of moon scan data from EVE (or any text containing a “Moon Product” column).
2. GooSniffer automatically:
    - Detects the clipboard contents
    - Parses the data and prints JSON to the console
    - If configured, uploads the JSON to your API endpoint

Example output:

```text
Listening for clipboard text. Press Ctrl+C to exit.
Clipboard changed
Possible moon scan data detected
Moon scan parsed:
{
  "66-PMM V - Moon 15": {
    "Flawless Arkonor": {
      "quantity": "0.323762148619",
      "ore_type_id": "46678",
      "solar_system_id": "30004923",
      "planet_id": "40311969",
      "moon_id": "40311985"
    }
  }
}
```

---

## 🧰 Build

```powershell
go build -o goosniffer.exe ./cmd/goosniffer
```

Requires **Go 1.23+** and Windows.