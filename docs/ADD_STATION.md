# Добавление модели станции

Инструкция для колонки, которой ещё нет в списке проверенных: что снять с
прошивки, как проверить аудиоконтракт и mDNS, как добавить профиль и как
собрать пакет. Добавляйте модель в существующий плагин: один Go-супервизор,
профиль аудио и подходящий receiver ABI. Новая запись в манифесте без проверки
PCM и библиотек может пройти установку и остаться без звука.

Правила доступа к устройству и публикации: [CONTRIBUTE.md](../CONTRIBUTE.md).
Границы того, что вообще проверено на живых моделях: [SHARED_MODELS.md](SHARED_MODELS.md)
и [UNVERIFIED_MODELS.md](UNVERIFIED_MODELS.md).

## 0. Что понадобится

- доступ к колонке (SSH или ADB) и возможность её перезагрузить;
- YAM и Luna. Для публикуемого пакета нужна `yam/luna >= 0.2.29`: 0.2.28
  принимает пакет с тремя плагинами, а 0.2.29 сохраняет аргументы демона
  при перезапуске после установки;
- Linux или WSL с Go 1.23+ для тестов (на macOS тесты не выполняются);
- UPX 5.2.1 для релизной сборки;
- Mac или Linux в той же сети для проверки mDNS.

Для отладки на колонке **не нужен** полный релизный пакет: достаточно пакета
приёмника (шаг 6). Панель и helper — это поставка, а не инструмент проверки.

## 1. Определить модель и окружение

Соберите model ID из API хоста, прошивку, архитектуру и аудиоконфигурацию.

Luna:

```sh
curl -s http://127.0.0.1:8099/api/device
```

YAM v2 (для установленного стека):

```sh
yam --platform <model> --profiles /data/yam/platforms list
```

Старый клиент YAM: `yam device`, `yam ps`. В SSH на самой станции:

```sh
id
uname -m
cat /system/vendor/quasar/version_full
cat /proc/asound/cards
cat /etc/asound.conf
od -An -tx1 -N 20 /usr/lib/libasound.so.2
```

Файл ALSA может лежать в другом месте: у оригинального Мини это
`/system/vendor/quasar/libs/libasound.so.2`, а действующая конфигурация — в
`/tmp/alsa/asoundrc`. Не выгружайте все пользовательские конфиги ради одного
значения.

Проверьте ELF-class и machine штатной ALSA и инструментов через
`readelf -h/-l/-d/-V`. `uname -m` показывает архитектуру ядра, а userspace
может быть ARMhf.

| ELF-class / machine | Receiver | Что требуется |
|---|---|---|
| 32-bit / ARM (`01`, `28 00` в заголовке) | `bin/shairport-sync-ap2` | ARMhf-загрузчик и совместимая штатная ALSA |
| 64-bit / AArch64 (`02`, `b7 00`) | `bin/shairport-sync-ap2-aarch64` | Штатные AArch64-библиотеки; shared-controls отдельно не квалифицированы |

Не подменяйте model ID в `device.toml` и не расширяйте allowlist догадками.
Не читайте `/sys/class/efuse` и `/sys/class/unifykeys`: на Мини даже чтение
приводило к зависанию до отключения питания.

## 2. Подтвердить общий аудиовыход

Оставьте Quasar/`maind`/`mediad` работающими. Возьмите имя PCM из реальной
конфигурации ALSA и проверьте секунду тишины, заменив пример `plug:dmix_spk`
на найденное имя:

```sh
dd if=/dev/zero bs=192000 count=1 | \
  aplay -D 'plug:dmix_spk' -f S16_LE -r 48000 -c 2 -t raw
printf 'APLAY:%s\n' "$?"
```

Нулевой код означает только успешную запись отсчётов. Затем проверьте
библиотечное окружение приёмника: системный `aplay` может иметь пути Quasar,
которых у приёмника нет.

| Результат | Следующее действие |
|---|---|
| Общий PCM работает и в окружении receiver | Задайте `pcm`, `quasarCoexist: true`; начните без индикации. |
| Работает только системный `aplay` | Исследуйте ABI и загрузчик; при доказанной несовместимости выберите выход, который смешивается с Алисой, как прямой `speakers` у Миди. |
| Общего PCM нет | Остановите адаптацию и согласуйте иной режим. Эксклюзивный `hw:0,0` может лишить Алису звука. |

Прямые профили обычно рассчитаны на 48 кГц/S16_LE/стерео; Миди — исключение:
её штатный микшер работает на 96 кГц. Успех старого receiver на 44,1 кГц не
подтверждает тот же контракт нового. На Мини `plug:` проходит с системным
`aplay`, но не с приёмником из-за конвертера libsamplerate; его профиль
открывает `softvol` напрямую.

## 3. Подтвердить имя mDNS

С Mac в той же сети:

```sh
dns-sd -B _yandexio._tcp local.
```

Возьмите имя из списка и разрешите сервис:

```sh
dns-sd -L 'точное имя из списка' _yandexio._tcp local.
```

Сравните объявленный hostname с `mdnsHostPrefix` и источником ID профиля.
Обычная схема — префикс плюс boot serial. У Мини — `yandexmini-<mv_uuid>` из
Linkplay-конфигурации. Серийный номер и `mv_uuid` не публикуйте.

Avahi должен добавлять сервисы к уже существующему hostname, с
`publish-addresses=no`. Неверный префикс вызывает конфликт имён; второй
независимый announcer не исправляет профиль. Текущий generated-конфиг ограничен
`wlan0`: другую сеть включайте только после подтверждения интерфейса и причины
сбоя.

## 4. Выбрать путь и индикацию

| Условие профиля | Что запускается | Ограничение |
|---|---|---|
| `pcm` задан, `ui: false` | Прямой shared ALSA | Для аудио часы и LED не обязательны. |
| `pcm` задан, `ui: true` | Shared ALSA плюс копия PCM для renderer | Нужны проверенная геометрия и API. На Лайт 2, Мини 3 и Миди shared-визуализатор уступает индикацию Алисе по состоянию контроллера. |

Для нового устройства начните с `ui: false`. Shared-controls включайте
отдельной проверкой: рабочий звук не подтверждает D-Bus transport, речь,
громкость или кнопку.

Старый `ui` требует объявленных `display.clock.segment.v1`, `led.ring.rgb`,
`led.ring.<N>` и точной раскладки сегментов. На Лайт 2 цифры `0..3` и
`DH`/`DL`; Миди использует цифры `1..4`. Не угадывайте геометрию по названию
модели.

## 5. Добавить профиль, матрицу и тесты

До изменения реализации добавьте тесты поведения: точный выбор модели, отказ
неизвестной модели, ABI, PCM, частота, hostname и отсутствие утечки
runtime-токена. Затем измените таблицу `profiles` в `source/service.go` и при
необходимости запуск в `source/profiles.go`.

| Поле | Откуда брать |
|---|---|
| `models` | Точный model ID из API и реально используемые алиасы |
| `name` | Читаемое имя приёмника без личных констант |
| `pcm`, `alsaLib`, `receiverABI` | Проверка микшера и ELF/loader |
| `mdnsHostPrefix`, `nvramHostID` | Фактическое штатное mDNS-объявление |
| `packagedDBus` | Наличие штатных daemon/send на прошивке |
| `airplayModel` | Согласованное представление в AirPlay: `AudioAccessory5,1` для компактных, `AudioAccessory1,1` для полноразмерных моделей |

Готовые профили: [SHARED_MODELS.md](SHARED_MODELS.md). Используйте
существующий receiver для нужной ABI.

Матрица целей лежит в [`release/yampkg/targets.json`](../release/yampkg/targets.json).
На цель там заданы model ID (в том числе `models_append`), начальный
`config` (голос, громкость, ducking, индикация, `spectrum`), `controls_config`,
`sleep`, `dbus_export`, `mango_guard` и дополнительные файлы. Новая модель
добавляется и в `profiles`, и в матрицу — иначе `package_yampkg.py` не выпустит
для неё пакет. Описание полей матрицы: [release/yampkg/README.md](../release/yampkg/README.md).

Версия в `plugin.toml` и в `version` (`source/service.go`) должна совпадать:
это проверяет `TestVersionMatchesManifest`, иначе `station-airplay --version`
врёт о сборке.

Тестовые образцы: `TestSharedProfilesKeepProvenAudioContract`,
`TestLight2SharesMixerAndHoldsDisplay`, `TestStockMDNSHostname`,
`tests/test_package.py` и `tests/test_package_yampkg.py`. Для прямого PCM
проверяйте отсутствие неожиданного `hw:0,0` или `spk_tee`.

## 6. Собрать

Тесты и vet выполняются на Linux; кросс-компиляция на macOS не значит, что
тесты прошли. Подробности инструментов и полной пересборки runtime:
[BUILD.md](../source/BUILD.md).

### Супервизор

```sh
cd source
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -buildvcs=false -trimpath -ldflags='-s -w' -o /tmp/station-airplay .
upx --best --lzma /tmp/station-airplay
```

### Пакет приёмника для отладки

`package_yampkg.py` пересобирает модельные `.yampkg` из опубликованного
базового релиза и берёт версию из аргумента:

```sh
python3 scripts/package_yampkg.py \
  --base <каталог базового релиза> \
  --binary /tmp/station-airplay \
  --version <версия> --slim-runtime --upx upx \
  --out /tmp/pkgs --targets <target>
```

Этого достаточно, чтобы проверить звук, mDNS, громкость, остановку процессов и
холодную загрузку. Панель и helper для этого не нужны.

### Релизный пакет (с 2.1.8 — один на три плагина)

Релиз собирается из пакета приёмника и helper-пакета той же цели:

```sh
# helper для цели
yam-publish pack --source <stage с panel/command.toml и бинарником panel-command> \
  --platform <target> --profiles <путь к platforms> \
  --output <base>/airplay-panel-command-<версия>-<target>.yampkg

# единый пакет: приёмник + панель + helper + postinstall
python3 scripts/build-release-bundle.py \
  --base-packages <base> --out <out> --luna-min 0.2.29
```

Сборщик переписывает версии дескрипторов и манифест, встраивает панель и
`postinstall`, затем читает каждый архив обратно и сверяет хэши, режимы и
набор плагинов. Проверка:

```sh
cd <out> && sha256sum -c SHA256SUMS
```

## 7. Установить на колонку

Сначала остановите прежний экземпляр и дождитесь исчезновения дочерних
процессов. **Пока запущен любой плагин из YAM-пакета, включая Lua-панель, Luna
держит общий flock на `/tmp/yam/yam.lock`**, и установка падает с
`another yam transaction is running`. Остановите приёмник и панель:

```sh
curl -s -X POST -H 'Content-Type: application/json' -d '{}' \
  http://127.0.0.1:8099/api/plugins/airplay.airplay-luna/stop
curl -s -X POST -H 'Content-Type: application/json' -d '{}' \
  http://127.0.0.1:8099/api/plugins/airplay.panel/stop
pidof station-airplay   # должно быть пусто
```

Дальше — установка файлом или из реестра:

```sh
yam install ./station-airplay-<версия>-<target>.yampkg
# или, если пакет уже опубликован:
yam install airplay/airplay-luna
```

Релизный пакет требует `yam/luna >= 0.2.29`. Если Luna старше, сначала
`yam install yam/luna`. После установки Luna должна включить
`airplay.airplay-luna`; панель и helper включаются фоном. С 2.1.11 обновление
можно делать кнопкой «Обновить AirPlay» в панели — она сама останавливает
плагины и возвращает их после установки. Команды legacy-YAM и Luna v2
различаются: [INSTALLATION.md](INSTALLATION.md).

## 8. Принять результат на колонке

Проверьте [сценарии владельца](CHECK_ON_YOUR_SPEAKER.md), а также:

- один супервизор, один receiver, один NQPTP; штатная Алиса продолжает работать;
- mDNS с другого устройства, достижимый hostname, затем реальная музыка;
- stop/disable завершают принадлежащие плагину процессы и освобождают порты;
- повторное включение восстанавливает обнаружение и звук;
- новый runtime не меняет защищённые конфиги и не требует старого личного launcher;
- мультирум, кнопки, экран и холодная загрузка отмечены отдельно как
  проверенные или непроверенные.

Что приложить к отчёту, чтобы модель можно было внести в матрицу:

- model ID из `/api/device` (или `yam device`) и версия прошивки;
- `uname -m` и `readelf -h` для `libasound.so.2` и приёмника;
- `cat /proc/asound/cards`, релевантные строки `/etc/asound.conf`;
- результат `aplay` и `printf 'APLAY:%s\n' "$?"` из шага 2;
- вывод `dns-sd -B` / `dns-sd -L` из шага 3;
- хвост лога плагина (`/data/yam/var/luna/data/plugins/plugin-state/airplay.airplay-luna/runtime.log`
  или лог в интерфейсе Luna);
- что проверено, а что нет: звук, Алиса, громкость, кнопка, экран, мультирум,
  холодная загрузка.
