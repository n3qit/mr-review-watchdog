# mr-review-watchdog

Утилита проверяет заданный список репозиториев GitLab на наличие merge request'ов,
которые открыты более суток и не набрали минимально необходимое количество ревью.
Найденные МР публикуются списком в канал Mattermost, ошибки — реплаем в тред под
этим сообщением. Бинарник запускается разово (по расписанию через внешний
cron/scheduler), демоном не является.

## Требования

- Go 1.24+
- GitLab EE с доступом к REST API v4 и personal access token с правом на чтение
  проектов, merge requests, approvals и notes.
- Mattermost с ботом (personal access token), имеющим право постить в целевой канал.

## Сборка

```sh
go build -o bin/mr-review-watchdog ./cmd/mr-review-watchdog
```

или

```sh
make build
```

## Конфигурация

Конфигурация передаётся флагом `--config` и представляет собой YAML-файл:

```yaml
gitlab:
  base_url: "https://gitlab.example.com"
  token: "glpat-xxxxxxxxxxxxxxxxxxxx"

mattermost:
  base_url: "https://mattermost.example.com"
  token: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"   # bot/personal access token
  channel_id: "abcd1234channelid"

check:
  min_reviewers: 2      # по умолчанию 2
  min_age_hours: 24     # по умолчанию 24

repositories:
  - group/project-a
  - group/subgroup/project-b
```

Обязательные поля: `gitlab.base_url`, `gitlab.token`, `mattermost.base_url`,
`mattermost.token`, `mattermost.channel_id`, непустой список `repositories`.
При отсутствии любого из них программа завершится с ошибкой ещё до начала
проверки.

## Запуск

```sh
./bin/mr-review-watchdog --config config.yaml
```

Периодичность запуска задаётся внешним планировщиком (cron, systemd timer и т.п.) —
собственного демона/планировщика утилита не содержит.

## Логика проверки

Для каждого репозитория из конфигурации:

1. Получаются все МР в состоянии `opened` (с пагинацией по GitLab API).
2. Исключаются draft МР (`draft == true` или legacy `work_in_progress == true`).
3. Исключаются МР младше `min_age_hours` часов.
4. Для оставшихся МР считается количество уникальных ревьюеров — пользователей,
   отличных от автора, поставивших approve и/или оставивших
   комментарий/обсуждение (системные заметки не учитываются).
5. Если ревьюеров меньше `min_reviewers` — МР считается проблемным.

Ошибка при обработке одного репозитория или отдельного МР (недоступен API,
таймаут и т.п.) не прерывает проверку остальных — она добавляется в отдельный
список ошибок.

## Уведомления в Mattermost

- Если найден хотя бы один проблемный МР — в канал отправляется корневое
  сообщение со списком всех проблемных МР.
- Если в ходе прогона были ошибки:
  - при наличии корневого сообщения — ошибки уходят реплаем в его тред;
  - если проблемных МР не найдено — ошибки уходят обычным сообщением в канал.
- Если нет ни проблемных МР, ни ошибок — сообщения не отправляются.

## Архитектура

```
cmd/mr-review-watchdog/main.go   — точка входа, оркестрация (без бизнес-логики)
internal/config           — структуры конфигурации, загрузка и валидация YAML
internal/gitlab           — клиент GitLab API v4: МР, approvals, notes/discussions
internal/checker          — фильтрация draft/возраста, подсчёт ревьюеров, отчёт
internal/mattermost       — клиент Mattermost Posts API: CreatePost, CreateReply
```

## Тестирование

```sh
go test ./...
```

или

```sh
make test
```

Юнит-тесты покрывают:

- фильтрацию draft/возраста и подсчёт уникальных ревьюеров (`internal/checker`, на моках GitLab-клиента);
- сборку текста корневого сообщения и сообщения об ошибках (`internal/checker`);
- клиенты GitLab и Mattermost (`internal/gitlab`, `internal/mattermost`, на `httptest`);
- оркестрацию отправки в Mattermost (`cmd/mr-review-watchdog`, на моках): реплай с
  корректным `root_id`, обычное сообщение без `root_id` при отсутствии
  проблемных МР, отсутствие вызовов при отсутствии МР и ошибок.

## Логирование

Структурированный (JSON) лог в stdout на уровнях info/error: какие репозитории
обработаны, сколько МР проверено, сколько проблемных найдено, какие ошибки
возникли. Токены в лог не попадают.
