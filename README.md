# Audio Text Manager Desktop

Десктопное приложение для **macOS** и **Windows**: локальная транскрибация через **whisper.cpp** и саммари через **llama.cpp**. Отдельный репозиторий от веб-сервиса Audio Text Manager.

Стек — [Wails v2](https://wails.io): нативное окно (WebView) и бэкенд на Go. Python, Ollama и Homebrew на машине пользователя **не нужны**, если собрать `make dist`. HTTP API на `:8000`, Kubernetes и Make-сервер из веб-версии не входят.

Версия продукта: `0.1.0` (`wails.json`).

Качество и скорость ASR **не 1:1** с faster-whisper из веб-ATM. Пресеты — квантованные ggml (Q5): быстрый small и средний medium.

## Что умеет

- Загрузка файлов: `wav`, `mp3`, `m4a`, `flac`, `ogg`, `mp4` (drag-and-drop или диалог выбора).
- Из видео берётся **первая аудиодорожка** `0:a:0`; исходный `mp4` после извлечения удаляется из `uploads/`.
- Нормализация через ffmpeg: **моно PCM WAV, 16 kHz**.
- Язык речи выбирается отдельным списком на форме (и при перезапуске). По умолчанию **русский** (`whisper-cli -l ru`). Есть автоопределение и основные европейские / азиатские языки.
- Пресеты ASR (веса ggml в бандле после `make dist`, иначе качаются при первом использовании):

  | Пресет UI | Ключ | Файл весов | Размер |
  | --- | --- | --- | --- |
  | Очень быстро (small Q5) | `fast` | `ggml-small-q5_1.bin` | ~181 МБ |
  | Средне (medium Q5) | `medium` | `ggml-medium-q5_0.bin` | ~514 МБ |

- Пресеты саммари: `gist` (о чём речь), `executive` (резюме для руководства), `meeting` (полный доклад). Либо свой промпт — тогда пресет игнорируется, один запрос к встроенному llama-server.
- История задач, стадии `upload / asr / summarize`, транскрипт доступен до конца саммари.
- Стоп текущей, стоп всех, удаление выбранных (аудио и результаты с диска без восстановления).
- Перезапуск полного пайплайна с другими параметрами.
- **Только саммари** по уже сохранённому тексту — Whisper повторно не запускается.
- Копирование и выгрузка транскрипта/саммари в `.txt` и `.doc` через системный диалог «Сохранить как».
- Настройки: найденные sidecar-пути, опциональные переопределения, кнопка «Проверить runtime».

Лимиты по умолчанию: аудио до **500 МБ**, видео до **4 ГБ**. Ограничение длительности (`max_audio_duration_sec`) по умолчанию выключено (`0`).

## Самодостаточный бандл

На чистом MacBook **не нужны** Ollama, ffmpeg, whisper-cli и Homebrew. В `.app` после `make dist` лежат:

| Что | Куда в `.app` |
| --- | --- |
| `ffmpeg`, `ffprobe` | `Contents/Resources/sidecar/ffmpeg/` |
| `whisper-cli` | `Contents/Resources/sidecar/whisper/` |
| `llama-server` | `Contents/Resources/sidecar/llama/` |
| ASR ggml + GGUF саммари | `Contents/Resources/models/` |

Модели:

| Роль | Файл | ~размер |
| --- | --- | --- |
| ASR fast | `ggml-small-q5_1.bin` | 181 МБ |
| ASR medium | `ggml-medium-q5_0.bin` | 514 МБ |
| Саммари | `qwen2.5-3b-instruct-q4_k_m.gguf` | 2.0 ГБ |

Итоговый `.app` ≈ **2.9 ГБ**. Системный WebView (WKWebView / WebView2) остаётся от ОС.

Сборка коробки (на машине разработчика нужны Go, Wails, cmake, git, интернет):

```bash
./build_macos.sh
open dist/AudioTextManager.app
```

Готовый бандл и zip — в `dist/`. Подробности: [BUILD.md](BUILD.md).  
`make dist` = sidecar + `wails build` + упаковка в `build/bin/`. `./build_macos.sh` делает то же и копирует очищенный `.app` + zip в `dist/`.

Если модели не упаковали, приложение при первой задаче скачает недостающие веса в `~/Library/Application Support/AudioTextManager/models/`. Бинарники без бандла оно само не установит.

## Что нужно пользователю

Только macOS (Apple Silicon или Intel — сборка под ту же архитектуру) или Windows с WebView2. Ollama не используется.

LLM: Qwen2.5 3B Instruct Q4 — лёгкая модель, нормально тянет gist и короткое executive.

## Сборка из исходников (разработка)

- [Go](https://go.dev/dl/) 1.22+ (в `go.mod` указан 1.25).
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation).
- cmake и git — чтобы собрать `whisper-cli` на macOS (`make fetch-runtime`).
- На целевой ОС: Xcode Command Line Tools / WebView2.

Для разработки можно `make fetch-runtime && wails dev` — sidecar ищется в `third_party/`.

## Быстрый старт (уже собранное `make dist`)

```bash
# macOS, из корня репозитория
open build/bin/AudioTextManager.app
```

Windows: запустите `build/bin/AudioTextManager.exe`.

Первый запуск:

1. Откройте **Настройки** → **Проверить runtime** (ffmpeg, whisper-cli, llama-server).
2. Закиньте короткий файл; для проверки пресет ASR — **Очень быстро (small Q5)**.
3. Дождитесь стадий Файл → Транскрибация → Саммари (первый LLM-запуск поднимает llama-server и может занять десятки секунд).

Для разработки без пересборки `.app`:

```bash
export PATH="$HOME/go/bin:$PATH"   # если `wails` не находится
wails dev
# или: make dev
```

## Установка Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

`go install` кладёт бинарь в `$GOBIN` или `$GOPATH/bin` (часто `~/go/bin`). Если после установки `wails: command not found`, каталог не в `PATH`. Для текущей сессии:

```bash
export PATH="$HOME/go/bin:$PATH"
```

Чтобы это сохранилось, добавьте ту же строку в `~/.zshrc` (macOS) или в профиль Windows.

Проверка: `wails version` (ожидается v2.x, в проекте используется `v2.16.0`).

## Сборка

Кросс-компиляция WebView **не поддерживается** — собирайте на целевой ОС.

```bash
make test          # go test ./internal/...
make dist          # самодостаточный .app (~2.9 ГБ)
# или по шагам:
make fetch-runtime
make build
make package
```

Артефакты:

| ОС | Путь |
| --- | --- |
| macOS | `build/bin/AudioTextManager.app` |
| Windows | `build/bin/AudioTextManager.exe` + каталоги `sidecar/` и `models/` рядом |

Подпись и нотаризация Apple — отдельный шаг, не блокер прототипа. NSIS-инсталлятор Windows можно добавить позже (`wails build -nsis`).

Бинарники и веса **не коммитятся**. Подробнее: [`third_party/README.md`](third_party/README.md).

Порядок поиска (первый найденный побеждает):

1. Явный путь из настроек.
2. `ATM_WHISPER_BIN`, `ATM_FFMPEG_BIN`, `ATM_FFPROBE_BIN`, `ATM_LLAMA_BIN`, `ATM_LLAMA_MODEL`.
3. `Contents/Resources/sidecar/{ffmpeg,whisper,llama}` и `Contents/Resources/models`.
4. `third_party/...` относительно рабочей директории (`wails dev`).
5. `PATH`.

## Как это работает

Один воркер в фоне берёт задачи из SQLite (опрос ~400 мс). Статусы: `queued` → `processing` → `done` / `error` / `canceled`.

```
файл → ffmpeg (WAV 16 kHz mono)
     → whisper-cli -m ggml-*.bin -f ….wav -nt -otxt
     → llama-server /v1/chat/completions (пресет или свой промпт)
```

Транскрипт для LLM обрезается примерно до **120 000 рун**. Параметры: `temperature=0.2`, контекст 8192, `max_tokens=2048`, таймаут чата 15 минут. `llama-server` поднимается лениво при первом саммари и живёт до закрытия окна.

Пресеты саммари отвечают **на языке транскрипта**. Если заполнен свой промпт, системная инструкция пресета не используется.

Алиасы размера саммари (для совместимости): `short` → `gist`, `medium` → `executive`, `long` → `meeting`.

## Данные

Приложение **не пишет** в Program Files и не модифицирует `.app`. Всё пользовательское — в каталоге данных:

| ОС | Каталог |
| --- | --- |
| macOS | `~/Library/Application Support/AudioTextManager/` |
| Windows | `%APPDATA%\AudioTextManager\` |
| Linux (dev) | `$XDG_DATA_HOME/AudioTextManager` или `~/.local/share/AudioTextManager` |

Содержимое:

| Файл / папка | Назначение |
| --- | --- |
| `app.db` | SQLite (WAL): очередь, статусы, транскрипт, саммари |
| `uploads/` | Нормализованные WAV задач |
| `models/` | Кэш ggml/GGUF, если их не было в бандле |
| `config.json` | Сохранённые настройки |

Путь к каталогу данных показан в модалке «Настройки».

### `config.json`

Поля (все опциональны; пустые пути = sidecar / PATH):

```json
{
  "llama_bin": "",
  "llama_model": "",
  "max_upload_bytes": 524288000,
  "max_video_upload_bytes": 4294967296,
  "whisper_bin": "",
  "ffmpeg_bin": "",
  "ffprobe_bin": "",
  "whisper_models_dir": "",
  "max_audio_duration_sec": 0
}
```

`whisper_models_dir` по умолчанию — `…/AudioTextManager/models`.

## Разработка

UI живёт в готовом `frontend/dist` (порт веб-ATM). Отдельного `npm install` / `frontend:build` нет: `wails.json` оставляет install/build команды пустыми, ассеты эмбеддятся через `//go:embed all:frontend/dist`.

Бэкенд-биндинги: `window.go.main.App.*`

| Метод | Назначение |
| --- | --- |
| `SelectAudioFile` | Нативный диалог выбора файла |
| `SaveExport` | Системный диалог сохранения `.txt` / `.doc` |
| `CreateJob` | Новая задача |
| `ListJobs` / `GetJob` | История и статус |
| `GetTranscript` / `GetResult` | Текст и финальный результат |
| `CancelJob` / `CancelActive` | Стоп одной / всех активных |
| `BulkDelete` | Удаление выбранных |
| `Requeue` | Полный перезапуск |
| `SummarizeOnly` | Только LLM по сохранённому транскрипту |
| `GetSettings` / `SaveSettings` | Настройки |
| `PingRuntime` | Проверка ffmpeg / whisper / llama-server |
| `DataDir` | Каталог данных |

Пакеты без WebView:

```
internal/appdir    каталог данных
internal/config    config.json
internal/jobs      SQLite
internal/media     ffmpeg / лимиты / WAV
internal/asr       whisper.cpp + веса ggml
internal/llm       llama-server + GGUF
internal/summary   chat completions
internal/sidecar   поиск бинарников и моделей
internal/pipeline  воркер очереди
internal/download  скачивание файлов
```

```bash
make test
# или
go test ./internal/...
```

CI (`.github/workflows/ci.yml`) гоняет тесты на `macos-latest` и `windows-latest`.

## Типичные проблемы

**`zsh: command not found: wails`**  
CLI установлен через `go install`, но `~/go/bin` не в `PATH`. См. [Установка Wails CLI](#установка-wails-cli).

**Окно открылось, транскрибация падает с «whisper.cpp CLI not found»**  
Собрали `wails build` без `make package`. Нужен `make dist`.

**Саммари в ошибке, транскрипт есть**  
Нет `llama-server` или GGUF в бандле. Соберите `make dist`. Транскрипт сохраняется — после починки runtime можно «Только саммари…».

**«no audio track in file»**  
В `mp4` нет аудиодорожки или ffmpeg не видит поток `0:a:0`.

**Первая саммари долго стартует**  
Поднимается `llama-server` и грузится Qwen 3B (~2 ГБ в RAM). Следующие задачи быстрее.

**Первая задача без бандла моделей качает веса**  
Если `Resources/models` пустой, качается ggml/GGUF в Application Support. После `make dist` качать не нужно.

**ld: warning … built for newer macOS version**  
Предупреждение линковщика при сборке, на запуск прототипа обычно не влияет.

**Gatekeeper / «приложение из интернета»**  
Сборка self-signed (ad-hoc). Для передачи своим см. [BUILD.md](BUILD.md). Для раздачи «двойной клик без вопросов» нужна нотаризация Apple.

## Лицензия и соседние репозитории

Это десктопный клиент, не замена веб-ATM. Веса Whisper — по условиям моделей на Hugging Face; Qwen2.5 — Apache-2.0. ffmpeg — LGPL/GPL в зависимости от сборки.
