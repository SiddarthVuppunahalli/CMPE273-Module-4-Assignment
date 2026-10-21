# Screenshot capture guide

Use three PowerShell terminals in the repository root: **A: Inventory**, **B: Order**, and **C: Requests**. In VS Code, split the terminal panel so all three can be visible together. Alternatively, arrange three terminal windows side by side. Increase the terminal panel height and use a readable font size so commands, status codes, and JSON responses fit.

These screenshots will show a new manual run. Their order IDs and timestamps may differ from the earlier automated transcript in `docs/verification.txt`.

## Preparation

Stop any copies of these services that you previously started using Ctrl+C in their terminals. Start fresh so the first order ID is `order-1`. Do not run the automated verification script during this capture sequence; the steps below deliberately leave the services running for screenshots.

Open the repository folder in each terminal:

```powershell
cd C:\Users\sidda\SJSU\273\CMPE273-Module-4-Assignment
```

In terminal A:

```powershell
$env:INVENTORY_ADDR = '127.0.0.1:8081'
go run ./cmd/inventory
```

Wait for `Inventory Service listening on http://127.0.0.1:8081`.

In terminal B:

```powershell
$env:ORDER_ADDR = '127.0.0.1:8080'
$env:INVENTORY_URL = 'http://127.0.0.1:8081'
go run ./cmd/order
```

Wait for `Order Service listening on http://127.0.0.1:8080`. Leave terminals A and B running.

In terminal C, prepare two reusable request files:

```powershell
$availableRequest = Join-Path $env:TEMP 'store-available.json'
$unavailableRequest = Join-Path $env:TEMP 'store-unavailable.json'
'{"item_id":"item-1","quantity":2}' | Set-Content -Encoding ascii $availableRequest
'{"item_id":"item-2","quantity":1}' | Set-Content -Encoding ascii $unavailableRequest
```

Keep using this same terminal C so these variables remain available.

## Screenshot 1: Successful order

In terminal C:

```powershell
Clear-Host
Get-Content $availableRequest
curl.exe -sS -i --max-time 5 -H 'Content-Type: application/json' --data-binary "@$availableRequest" http://127.0.0.1:8080/orders
```

Expected: `201 Created`, `Location: /orders/order-1`, and JSON with `"id":"order-1"` and `"status":"created"`.

Capture all three terminal panes so both service startup messages and the successful response are visible. Press **Win+Shift+S**, select the area, then open the captured snip and save it as:

```text
docs/screenshots/01-success.png
```

Use the actual returned ID for later lookups if it differs from `order-1`.

## Screenshot 2: Unavailable item

In terminal C:

```powershell
Clear-Host
Get-Content $unavailableRequest
curl.exe -sS -i --max-time 5 -H 'Content-Type: application/json' --data-binary "@$unavailableRequest" http://127.0.0.1:8080/orders
```

Expected: `409 Conflict` and `{"error":"requested item or quantity is unavailable"}`. Capture the command, request body, and response, then save:

```text
docs/screenshots/02-unavailable.png
```

## Screenshot 3: Inventory failure and continued Order operation

In terminal A, press **Ctrl+C** and wait for the PowerShell prompt to return. This stops Inventory. Keep terminal B running and visible; do not restart Order.

In terminal C:

```powershell
Clear-Host
Get-Content $availableRequest
curl.exe -sS -i --max-time 5 -H 'Content-Type: application/json' --data-binary "@$availableRequest" http://127.0.0.1:8080/orders
curl.exe -sS -i --max-time 5 http://127.0.0.1:8080/orders/order-1
```

Expected: the POST returns `503 Service Unavailable` with `inventory service unavailable; please try again later`. The GET then returns `200 OK` with the previously created order, proving Order is still responding during the outage.

Capture terminal A's returned prompt, terminal B still running, and both responses in terminal C. Make the terminal panel tall enough to show both responses. Save:

```text
docs/screenshots/03-inventory-failure.png
```

## Screenshot 4: Recovery

In terminal A, restart Inventory:

```powershell
go run ./cmd/inventory
```

Wait for its listening message. Leave the existing Order process in terminal B running.

In terminal C:

```powershell
Clear-Host
Get-Content $availableRequest
curl.exe -sS -i --max-time 5 -H 'Content-Type: application/json' --data-binary "@$availableRequest" http://127.0.0.1:8080/orders
```

Expected: `201 Created` with `order-2`, assuming you followed this sequence exactly. The increasing order ID also supports that Order retained its state across the Inventory outage.

Capture Inventory's restart, the unchanged Order terminal, and the successful response. Save:

```text
docs/screenshots/04-recovery.png
```

## Finish

Open the root README preview in VS Code with Ctrl+Shift+V and check that all four screenshots display at a readable size. The root README already contains the image references; no image links need to be edited when these filenames are used.

Stop both services with Ctrl+C in terminals A and B when finished. Review and stage the screenshots with the final documentation:

```powershell
git add README.md scripts/verify.ps1 docs/
git status --short
```

Commit after checking that all four PNG files are included. The existing captured terminal output remains useful alongside the screenshots.
