# Релиз 2.1.8: единый пакет AirPlay

Документ фиксирует, что уже готово к публикации и что нужно сделать после
выхода Luna с поддержкой пакетов из нескольких плагинов. Публикация
выполняется только по отдельному решению владельца.

## Что входит

- `airplay/airplay-luna <version>`: один модельный `.yampkg` на цель;
- внутри — приёмник `airplay.airplay-luna`, helper `airplay.panel-command`
  (`0.1.0`) и панель `airplay.panel` (`0.1.0`);
- зависимость `yam/luna >=<минимум>`; по умолчанию `0.2.27`;
- `conflicts`/`replaces` для старых `airplay/panel` и `airplay/panel-command`.

`postinstall` включает приёмник и панель, а helper оставляет включённым без
автозапуска: Luna хранит флаг `enabled` отдельно для каждого ID, и без этого
свежая установка показывала бы выключенные панель и helper.

Runtime-бинарник приёмника — UPX-супервизор из проверенной базы 2.1.8,
которая прошла живую проверку на четырёх колонках. Сборщик не меняет
payload: только версии дескрипторов, зависимость и манифест. Каждый архив
читается обратно, сверяются хэши, режимы и набор плагинов.

## Перед сборкой

1. Убедиться, что релиз Luna содержит:
   - пакет с несколькими плагинами и разрешение компонентов к владельцу
     payload (PR #39);
   - нативную громкость Станции 2 (PR #37) — иначе 2.1.8 не публиковать
     как «громкость Станции 2 исправлена».
2. Записать точный минимальный номер Luna в `--luna-min` (по умолчанию
   `0.2.27`). Если релиз получил другой номер — пересобрать с ним.
3. Проверить, что изменения оригинала Mini, которые должны войти в релиз,
   есть в базовом runtime. Если нужен другой супервизор — пересобрать базу
   и заново прогнать QEMU и живую проверку.

## Сборка

```sh
cd release/yampkg  # из корня пакета
python3 ../../scripts/build-release-bundle.py \
  --base-packages /mnt/foto/home-lab-dev/build-workspaces/airplay-plugin-panel-test2-20261115/packages \
  --out /mnt/foto/home-lab-dev/build-workspaces/airplay-release-2.1.8-20261115/packages \
  --luna-min 0.2.27
```

Выход: 12 архивов, `SHA256SUMS` и `release-provenance.json` с хэшами
базовых пакетов и коммитом исходников. Проверить:

```sh
cd /mnt/foto/home-lab-dev/build-workspaces/airplay-release-2.1.8-20261115/packages
sha256sum -c SHA256SUMS
python3 /mnt/foto/home-lab-dev/build-workspaces/luna-full2-rollout-20261115/qa-full2.py
```

QEMU-прогон для релиза должен ставить пакет поверх тестовой `yam/luna`
нужной версии и проверять три плагина, страницу, helper без автозапуска,
сохранение настроек, удаление и повторную установку.

## Публикация

Нужны token и Ed25519 key из профиля реестра; на колонку их не копировать.

```sh
export YAM_AUTHOR_KEY_FILE=~/Keys/yam-author-private-key.pem
export YAM_PUBLISH_TOKEN=...   # не печатать в лог и не коммитить

for target in bergamot cucumber mango orion pickle plum yandexmicro \
              yandexmidi yandexmini_2 yandexmini yandexplum yandexstation_2; do
  yam-publish release \
    --source <staged-source> \
    --platform "$target" \
    --profiles <yam>/platforms \
    --output "../packages/station-airplay-2.1.8-$target.yampkg" \
    --token "$YAM_PUBLISH_TOKEN" \
    --key "$YAM_AUTHOR_KEY_FILE"
done
```

`yam-publish` создаёт архив заново из исходников. Если публиковать готовые
архивы нельзя, исходное дерево должно давать побайтно тот же payload:
сверить `sha256sum` с `SHA256SUMS` до отправки.

## После публикации

1. `yam install airplay/airplay-luna` на одной колонке каждой семьи:
   Станция 2, Мини 2, Лайт 2. Проверить версию, три плагина, страницу.
2. Обновление с опубликованного 2.1.7 на той же колонке.
3. Перезагрузка: приёмник и панель поднимаются сами, helper не стартует.
4. Удаление и повторная установка; настройки и цвета сохраняются.
5. Откат: `yam install` предыдущего опубликованного 2.1.7.

## Ограничения

- Физический звук, обложки, громкость, мультирум и EQ проверяет владелец
  колонки; эмулятор подтверждает только установку, запуск, автозапуск и
  страницы.
- EQ и калибровочная панель остаются выключенными; `/calibration` — часть
  хоста Luna, не этого пакета.
- Mini 1 использует приватный loader/libc из базы; менять его в релизной
  сборке нельзя без отдельной проверки.
