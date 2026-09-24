# Сборка Audio Text Manager для macOS

Готовый бандл — одно окно `.app`: UI внутри, рядом в `Contents/Resources` лежат ffmpeg, whisper-cli, llama-server и веса. На машине пользователя не нужны Python, Node, Ollama, Homebrew и исходники.

Стек сборки: **Go + Wails v2** (не PyInstaller / py2app).

## Что нужно на машине разработчика

- macOS той же архитектуры, на которую собираете (Apple Silicon → `arm64`, Intel → `x86_64`)
- [Go](https://go.dev/dl/) (версия как в `go.mod`)
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- `export PATH="$HOME/go/bin:$PATH"`
- cmake и git — один раз, чтобы собрать `whisper-cli` (`make fetch-runtime`)
- интернет при первой загрузке sidecar и моделей

## Сборка

```bash
cd /path/to/audio-text-manager-desktop
./build_macos.sh
```

Скрипт вызывает `make dist` (sidecar + `wails build` + упаковка), чистит AppleDouble, разворачивает симлинки llama, подписывает ad-hoc и кладёт результат в `dist/`.

Эквивалент вручную: `export PATH="$HOME/go/bin:$PATH" && make dist`.

## Куда кладётся результат

| Файл | Назначение |
| --- | --- |
| `dist/AudioTextManager.app` | Приложение |
| `dist/AudioTextManager.zip` | То же, для передачи (симлинки и права сохранены, без `._*`) |

Копия Wails остаётся в `build/bin/AudioTextManager.app`. Для раздачи берите **`dist/`**.

Проверка у себя:

```bash
open dist/AudioTextManager.app
lipo -archs dist/AudioTextManager.app/Contents/MacOS/AudioTextManager
codesign --verify --deep --strict dist/AudioTextManager.app
```

## Как передать на другой Mac

1. Отправляйте **`AudioTextManager.zip`**, не папку `.app` в облако «как файлы».
2. Лучше флешка или AirDrop, чем веб-диск (меньше карантина).
3. На том Mac: распаковать **двойным кликом** или  
   `ditto -x -k --norsrc AudioTextManager.zip ~/Desktop`
4. Перетащить `.app` в `/Applications` или оставить на Рабочем столе.
5. Сборка **arm64** не запустится на Intel и наоборот. Для Intel соберите скрипт на Intel-Mac. Universal (оба чипа в одном `.app`) сейчас не собирается.

Данные задач пишутся в `~/Library/Application Support/AudioTextManager/`, не в папку проекта.

## Если macOS блокирует запуск

Сертификат Developer ID и нотаризация **не обязательны** для передачи своим. Ad-hoc подпись после скачивания из интернета часто даёт «неизвестный разработчик» или «повреждена».

**Сначала:** не в Корзину. ПКМ по `.app` → **Открыть** → снова **Открыть**.

Если пишет «повреждена» — карантин или мусор `._*` после облака. На **том** Mac:

```bash
APP="$HOME/Desktop/AudioTextManager.app"
find "$APP" -name '._*' -delete
xattr -cr "$APP"
codesign --force --deep --sign - "$APP"
open "$APP"
```

`xattr` из `~/Downloads` иногда не снимается (защита папки). Перетащите приложение на Рабочий стол и повторите.

Двойной клик «как из App Store» без этих шагов возможен только после нотаризации Apple (платный Developer Program). На этом этапе это не требуется.

## Разработка без упаковки

```bash
export PATH="$HOME/go/bin:$PATH"
make fetch-runtime
wails dev
```

Sidecar ищется в `third_party/`.
