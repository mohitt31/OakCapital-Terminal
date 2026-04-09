# CGO Engine Integration Guide (Sequential)

This is the single path to set up, validate, and use the C++ matching engine through Go CGO.

Follow the steps in order from top to bottom.

## 0) What connects to what

```
Go API handlers
  -> internal/api/book_manager.go
  -> internal/engine/cgo_bridge.go
  -> Matching-Engine/src/engine_c_api.cpp (extern "C")
  -> Matching-Engine/Limit_Order_Book/* (C++ engine core)
```

There is no network boundary between Go and C++. Calls are direct function calls through CGO.

## 1) Prerequisites

### macOS

- `brew install cmake go`
- `xcode-select --install`
- Verify:
  - `cmake --version`
  - `go version`
  - `go env CGO_ENABLED` should be `1`

### Linux

- `sudo apt update && sudo apt install -y cmake build-essential`
- Install Go from [https://go.dev/dl/](https://go.dev/dl/)
- Verify:
  - `cmake --version`
  - `go version`
  - `go env CGO_ENABLED` should be `1`

### Windows (recommended: MSYS2 UCRT64 + MinGW)

1. Install MSYS2: [https://www.msys2.org/](https://www.msys2.org/)
2. In MSYS2 UCRT64 terminal:

```bash
pacman -Syu
pacman -S mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-cmake mingw-w64-ucrt-x86_64-make
```

3. Add to system `PATH`:

```text
C:\msys64\ucrt64\bin
```

4. Open a new PowerShell/Command Prompt and verify:

```powershell
gcc --version
cmake --version
mingw32-make --version
go version
go env CGO_ENABLED
go env CC
```

If needed:

```powershell
[Environment]::SetEnvironmentVariable("CGO_ENABLED", "1", "User")
```

## 2) Build the C++ shared library

Run from repository root.

### macOS / Linux

```bash
cd Matching-Engine
mkdir -p build
cd build
cmake ..
make -j$(nproc)                # Linux
# make -j$(sysctl -n hw.ncpu)  # macOS alternative
mkdir -p ../../lib
cp libmatching_engine_c_api.so ../../lib/       # Linux
# cp libmatching_engine_c_api.dylib ../../lib/  # macOS
cd ../..
```

### Windows

```powershell
cd Matching-Engine
mkdir build
cd build
cmake .. -G "MinGW Makefiles"
mingw32-make -j$env:NUMBER_OF_PROCESSORS
mkdir ..\..\lib -ErrorAction SilentlyContinue
copy libmatching_engine_c_api.dll ..\..\lib\
cd ..\..
```

Command Prompt equivalent:

```cmd
cd Matching-Engine
mkdir build
cd build
cmake .. -G "MinGW Makefiles"
mingw32-make -j%NUMBER_OF_PROCESSORS%
if not exist ..\..\lib mkdir ..\..\lib
copy libmatching_engine_c_api.dll ..\..\lib\
cd ..\..
```

## 3) Run C++ smoke test

```bash
cd Matching-Engine/build
./matching_engine_smoke          # macOS/Linux
matching_engine_smoke.exe        # Windows
cd ../..
```

Expected output contains:

```text
best_bid=100
executed=1
```

## 4) Verify Go linkage to C++ library

Run from repository root.

### macOS / Linux

```bash
CGO_ENABLED=1 go build ./internal/engine
```

### Windows (PowerShell)

```powershell
$env:CGO_ENABLED="1"
$env:PATH = "$PWD\lib;$env:PATH"
go build ./internal/engine
```

### Windows (Command Prompt)

```cmd
set CGO_ENABLED=1
set PATH=%CD%\lib;%PATH%
go build ./internal/engine
```

If this command succeeds with no output, CGO linkage is working.

## 5) Run Go engine tests

### macOS / Linux

```bash
CGO_ENABLED=1 go test -v ./internal/engine
```

### Windows (PowerShell)

```powershell
$env:CGO_ENABLED="1"
$env:PATH = "$PWD\lib;$env:PATH"
go test -v ./internal/engine
```

### Windows (Command Prompt)

```cmd
set CGO_ENABLED=1
set PATH=%CD%\lib;%PATH%
go test -v ./internal/engine
```

Notes:

- `No sell/buy limit at X` lines are debug prints from C++ and are not fatal by themselves.
- Treat test status (`PASS`/`FAIL`) as the actual signal.

## 6) Start backend with integrated engine

```bash
go run ./cmd/server -port 8080
```

Available endpoints include:

- `/health`
- `/api/v1/order/limit/add`
- `/api/v1/order/market`
- `/api/v1/order/stop/add`
- `/api/v1/order/stop-limit/add`
- `/api/v1/book/info`
- `/api/v1/book/depth`
- `/api/v1/book/list`

## 7) Minimal API validation sequence

1. Add resting order:
   - `POST /api/v1/order/limit/add`
2. Submit crossing or market order:
   - `POST /api/v1/order/market`
3. Inspect state:
   - `GET /api/v1/book/info?symbol=...`
   - `GET /api/v1/book/depth?symbol=...`
4. Validate error path:
   - Cancel unknown order and confirm `code: ENGINE_ERR_NOT_FOUND`

## 8) Correct usage in Go code

### Create one book per symbol

```go
book := engine.New()
defer book.Close()
```

### Place orders

```go
status, result := book.AddLimit(orderID, engine.SideBuy, 100, 5000)
if status != engine.StatusOK {
    // handle error
}
_ = result
```

### Inspect trades and changes

```go
status, result := book.Market(orderID, engine.SideSell, 50)
if status == engine.StatusOK && len(result.Trades) > 0 {
    first := result.Trades[0]
    _ = first.Price
    _ = first.Qty
}
```

### Query book snapshot

```go
bestBid := book.BestBid()
bestAsk := book.BestAsk()
lastPrice := book.LastExecutedPrice()
execCount := book.LastExecutedCount()
depth := book.GetDepth()
_, _, _, _, _ = bestBid, bestAsk, lastPrice, execCount, depth
```

## 9) Integration rules you must keep

- Engine is not thread-safe per handle.
  - Serialize all calls per symbol/book (mutex or per-book actor loop).
- Prices are integers (use cents/ticks, not floating point).
- Order IDs must be unique per book for cancel/modify correctness.
- Always call `Close()` to free C++ memory.

## 10) Rebuild flow after C++ changes

Whenever `Matching-Engine/src` or `Matching-Engine/Limit_Order_Book` changes:

1. Rebuild shared library (Step 2)
2. Re-run smoke test (Step 3)
3. Re-run Go build/test (Steps 4 and 5)

## 11) Troubleshooting

### `ld: library 'matching_engine_c_api' not found`

Library not built or not copied to `lib/`. Re-run Step 2.

### `fatal error: 'engine_c_api.h' file not found`

Build command not run from repo root, or include paths broken.

### Windows: `libmatching_engine_c_api.dll was not found` / `0xc0000135`

Use PowerShell in repo root:

```powershell
$env:PATH = "$PWD\lib;$env:PATH"
go test -v ./internal/engine
```

Or use Command Prompt in repo root:

```cmd
set PATH=%CD%\lib;%PATH%
go test -v ./internal/engine
```

Also verify file exists:

```powershell
Test-Path .\lib\libmatching_engine_c_api.dll
```

Command Prompt equivalent:

```cmd
if exist .\lib\libmatching_engine_c_api.dll (echo found) else (echo missing)
```

### `exec: "gcc": executable file not found in PATH`

MinGW not on `PATH`. Add `C:\msys64\ucrt64\bin` and open a new terminal.

### `mingw32-make` not found

Install package in MSYS2 UCRT64:

```bash
pacman -S mingw-w64-ucrt-x86_64-make
```
