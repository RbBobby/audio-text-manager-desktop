# Сборка Audio Text Manager

Два комплекта (на каждой ОС — оба):

| Комплект | Модели Whisper | Спикеры | Каталог |
| --- | --- | --- | --- |
| `medium` | small + medium | нет | `dist/medium/` |
| `speakers` | small + large-v3 | да, на large-v3 | `dist/speakers/` |

В обоих есть Qwen для саммари. Кросс-компиляция WebView **не работает**: macOS собирайте на Mac, Windows — на Windows.

## Комплект medium

В меню: «Очень быстро (small)» и «Средне (medium)». Транскрипт сплошным текстом, без «Спикер N».

## Комплект speakers

В меню: small и «Точно (large-v3 Q5, спикеры)». На large-v3 реплики помечаются как Спикер 1 / Спикер 2. Small в этой сборке без разметки голосов.

---

## macOS

Нужны Go (версия из `go.mod`), [Wails v2](https://wails.io/docs/gettingstarted/installation), cmake, git, интернет.

```bash
cd /path/to/audio-text-manager-desktop
export PATH="$HOME/go/bin:$PATH"
./build_macos.sh          # оба комплекта
# или один:
./build_macos.sh medium
./build_macos.sh speakers
```

Результат:

- `dist/medium/AudioTextManager.app` и `.zip`
- `dist/speakers/AudioTextManager.app` и `.zip`

Отдавайте **zip**. Подробности про Gatekeeper — как раньше: ПКМ → Открыть.

---

## Windows (на ПК с Windows x64)

Не запускайте `build_windows.sh` с Mac.

### Один раз поставить

1. [Git for Windows](https://git-scm.com/download/win) — дальше все команды в **Git Bash**.
2. [Go](https://go.dev/dl/) (как в `go.mod`).
3. Wails:
   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
   Добавьте в PATH: `C:\Users\<вы>\go\bin` (Параметры → Переменные среды).
4. Компилятор для CGO, который просит Wails: `wails doctor` и поставьте то, что он напишет (часто MinGW / TDM-GCC).
5. [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) — на Windows 11 обычно уже есть.
6. Интернет на первую загрузку sidecar и моделей (~4 ГБ в кэше `third_party/`, в каждый zip пойдёт только свой набор Whisper).

Проверка:

```bash
go version
wails version
wails doctor
```

### Сборка

В Git Bash, из корня репозитория:

```bash
export PATH="$HOME/go/bin:$PATH"
./build_windows.sh          # оба комплекта
# или:
./build_windows.sh medium
./build_windows.sh speakers
```

Скрипт сам качает ffmpeg, whisper-cli, llama-server и модели, затем `wails build` и пакует два набора.

Результат:

```
dist/medium/AudioTextManager/          ← папка: exe + sidecar + models
dist/medium/AudioTextManager-windows.zip
dist/speakers/AudioTextManager/
dist/speakers/AudioTextManager-windows.zip
```

Пользователю отдайте **zip** или всю папку `AudioTextManager`. Запуск: `AudioTextManager.exe`. Рядом обязательно должны остаться `sidecar\` и `models\`.

Данные задач: `%AppData%\AudioTextManager\`.

### Если `wails` не находится

```bash
export PATH="$HOME/go/bin:$PATH"
```

или в cmd: `set PATH=%USERPROFILE%\go\bin;%PATH%`

### Если нет ffmpeg

`scripts/fetch-runtime.sh` качает essentials с gyan.dev. Если зеркало упало — вручную положите `ffmpeg.exe` и `ffprobe.exe` в `third_party/ffmpeg/` и снова `./build_windows.sh`.

---

## Что не коммитится

Веса и бинарники живут в `third_party/` на машине сборки. В git их нет.

Разработка без комплекта: `make fetch-runtime && wails dev` — в UI будут все модели, которые лежат в `third_party/models`.
