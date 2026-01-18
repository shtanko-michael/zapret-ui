# zapret-ui

GUI-приложение для управления стратегиями zapret (Wails + React/TypeScript).

<img width="994" height="816" alt="image" src="https://github.com/user-attachments/assets/1fdb76b6-5cc4-4179-8526-0999babcdc60" />

## Скачивание готовых версий

Готовые версии приложения публикуются в репозитории `Flowseal/zapret-discord-youtube`:
- Репозиторий: `https://github.com/Flowseal/zapret-discord-youtube`
- Releases: `https://github.com/Flowseal/zapret-discord-youtube/releases`

## Разработка (запуск)

### Требования

- Go (см. `go.mod`)
- Node.js + pnpm
- Wails CLI

Установка Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Запуск в dev-режиме

Запуск через Wails (Vite + hot reload):

```powershell
.\scripts\wails-dev.ps1
# или
wails dev
```

## Сборка (build)

### Обычная сборка

```powershell
wails build -platform windows/amd64
```

Артефакт по умолчанию: `build\bin\zapret-ui.exe`

### Windows UAC (dev vs release)

- Dev (`wails dev`) должен запускаться **без повышения прав**.
- Release-сборка должна **запрашивать права администратора** (UAC), т.к. действия с тестами/сервисом требуют этого.

Сборка release под Windows с манифестом `requireAdministrator`:

```powershell
.\scripts\wails-build-release.ps1 -platform windows/amd64
```

## Лицензия

MIT, см. файл `LICENSE`.
