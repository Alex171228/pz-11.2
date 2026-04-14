# Практическое задание 10
## Шишков А.Д. ЭФМО-02-22
## Тема
Горизонтальное масштабирование: балансировка нагрузки (NGINX).

## Цель
Освоить подход к горизонтальному масштабированию backend-приложения: несколько экземпляров сервиса tasks, входящий HTTP через NGINX как load balancer, health-проверки и демонстрация распределения запросов.

---

## 1. Горизонтальное и вертикальное масштабирование

**Вертикальное** — увеличение ресурсов одного узла (CPU, RAM, диск).

**Горизонтальное** — добавление **одинаковых** экземпляров приложения (tasks-1, tasks-2, tasks-3). Клиент обращается к **одному адресу** (NGINX), балансировщик направляет запросы на доступные backend’ы.

Преимущества горизонтального масштабирования: отказоустойчивость, рост пропускной способности без «потолка» одной машины.

---

## 2. Роль load balancer и NGINX

**Load balancer** распределяет входящие запросы по группе серверов. В работе используется **NGINX** как **reverse proxy**: принимает соединение от клиента, выбирает upstream-сервер, проксирует запрос и возвращает ответ.

---

## 3. Upstream в NGINX

**Upstream** — именованная группа backend-серверов. По умолчанию NGINX использует **round-robin**: запросы по очереди уходят на `tasks_1`, `tasks_2`, `tasks_3`.

Для устойчивости к кратковременным сбоям заданы **`max_fails`** и **`fail_timeout`**: после нескольких неудачных попыток upstream временно исключается из ротации.

Файл `deploy/lb/nginx.conf` (фрагмент):

```nginx
upstream tasks_backend {
    server tasks_1:8082 max_fails=3 fail_timeout=30s;
    server tasks_2:8082 max_fails=3 fail_timeout=30s;
    server tasks_3:8082 max_fails=3 fail_timeout=30s;
}

server {
    listen 8080;
    location / {
        proxy_pass http://tasks_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Request-ID $request_id;
    }
}
```

Имена `tasks_1`, `tasks_2`, `tasks_3` — это **имена сервисов** в Docker Compose (DNS внутри сети), не `localhost` на хосте.

---

## 4. Stateless-сервисы

При балансировке один и тот же пользователь может попасть на **разные** инстансы. Сессии **нельзя** хранить только в памяти одного процесса без общего хранилища.

В проекте: данные задач в **PostgreSQL**, кэш в **Redis** — общие для всех реплик tasks, что соответствует **stateless**-модели за вычетом внешних зависимостей (БД, Redis).

---

## 5. Идентификация инстанса: INSTANCE_ID и X-Instance-ID

- Переменная окружения **`INSTANCE_ID`** задаёт имя реплики (`tasks-1`, `tasks-2`, `tasks-3`).
- В каждом HTTP-ответе выставляется заголовок **`X-Instance-ID`** (включая `/metrics`), чтобы по логам и `curl -i` было видно, **какой инстанс** обработал запрос.
- **`GET /health`** возвращает JSON с полями `status`, `service`, **`instance`**.
- **`GET /whoami`** — дополнительный эндпоинт для демонстрации: `{"instance":"tasks-1"}` без авторизации.

**Доп. задание, вариант 2 — логирование с `instance_id`:** в каждой строке access log при завершении запроса (`request completed`) в JSON-логе есть поле **`instance_id`** (значение совпадает с `INSTANCE_ID` реплики). Реализация: `shared/middleware/accesslog.go` — у `AccessLog` добавлены опциональные поля; в **tasks** вызывается `AccessLog(log, zap.String("instance_id", instanceID))`, сервис **auth** по-прежнему использует `AccessLog(log)` без лишних полей.

Проверка: `docker compose logs -f tasks_1` (или `lb_tasks_1`) и несколько запросов через LB — в логах видно, какой инстанс обработал запрос.

Файлы: `services/tasks/cmd/tasks/main.go`, `shared/middleware/accesslog.go`, `InstanceIDMiddleware` и `handler.go` в `services/tasks/internal/http/`.

---

## 6. Структура стенда с балансировкой

```
deploy/lb/
  docker-compose.yml   — db, redis, tasks_1..3, nginx
  nginx.conf           — upstream + proxy
```

Сборка образа tasks — **корень репозитория**, как в основном compose:

```yaml
build:
  context: ../..
  dockerfile: services/tasks/Dockerfile
```

Снаружи публикуется только **порт 8080** (NGINX). PostgreSQL и Redis доступны только внутри сети compose (конфликта портов с `deploy/docker-compose.yml` нет).

**Auth** по-прежнему на хосте: `AUTH_BASE_URL=http://host.docker.internal:8081` и `extra_hosts: host.docker.internal:host-gateway`.

---

## 7. Запуск

```bash
cd deploy/lb
docker compose up -d --build
docker compose ps
```

---

## 8. Проверки

### Health и заголовок

```bash
curl -i http://localhost:8080/health
```

Ожидается `200`, в теле `"instance":"tasks-…"`, в заголовках **`X-Instance-ID`**.

![Проверка /health](docs/images/pz10_check_health.png)

### Whoami и round-robin

```bash
for i in 1 2 3 4 5 6; do curl -s http://localhost:8080/whoami; echo; done
```

В ответах должны чередоваться **`tasks-1`**, **`tasks-2`**, **`tasks-3`**.

![Round-robin /whoami](docs/images/pz10_check_whoami.png)

### API с Authorization через NGINX

```bash
TOKEN=$(curl -s http://localhost:8081/v1/auth/login -H "Content-Type: application/json" \
  -d '{"username":"student","password":"student"}' | jq -r .access_token)
curl -i http://localhost:8080/v1/tasks -H "Authorization: Bearer $TOKEN"
```

Заголовок **`X-Instance-ID`** меняется при повторных запросах.

![GET /v1/tasks через балансировщик](docs/images/pz10_check_tasks.png)

### Отказ одной реплики

```bash
docker compose stop tasks_1
# несколько запросов — только tasks-2 и tasks-3
for i in 1 2 3 4 5; do curl -sI http://localhost:8080/whoami | grep -i X-Instance-ID; done
docker compose start tasks_1
```

![Отказ реплики tasks_1](docs/images/pz10_check_failover.png)

### Логи с `instance_id` (доп. задание)

```bash
docker compose logs tasks_1 --tail 50
```

Несколько запросов через LB, затем в логах — **`instance_id`** в записи о завершении запроса.

![Логи с instance_id](docs/images/pz10_check_logs.png)

---

## 9. CI

В **GitHub Actions** добавлена проверка синтаксиса конфигурации NGINX: `nginx -t` в контейнере с примонтированным `deploy/lb/nginx.conf`. Для прохождения проверки в контейнер добавлены записи `--add-host=tasks_*:127.0.0.1`. Без реального upstream NGINX не резолвит имена при `nginx -t`.

Локально:

```bash
docker run --rm \
  --add-host=tasks_1:127.0.0.1 --add-host=tasks_2:127.0.0.1 --add-host=tasks_3:127.0.0.1 \
  -v "$(pwd)/deploy/lb/nginx.conf:/etc/nginx/nginx.conf:ro" \
  nginx:1.27-alpine nginx -t
```

