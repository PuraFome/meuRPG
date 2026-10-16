# Logs

The API writes one JSON line per event to stdout. On Cloud Run that goes to Cloud Logging; on a developer machine an optional ELK stack reads it. The fields, levels and what never goes in a line are defined once, in [Architecture](../architecture/logs.md#logs) (code: `backend/internal/platform/logging`); this page covers where the lines go and how to search them.

## On Cloud Run

Cloud Run sends stdout to Cloud Logging, which keeps it 30 days in the `_Default` bucket. Lines carry:

- `severity` (`WARNING` for warnings), `message`, and the fields of the call site;
- `request_id`, `user_id` and `campaign_id` when the request knows them (the request id is also the `X-Request-Id` response header; a client-sent id is never trusted);
- `logging.googleapis.com/trace`, which joins the lines of one request in Cloud Logging, only when `GOOGLE_CLOUD_PROJECT` is set (Cloud Run does not set it; the deploy does).

The load balancer's own request log (client IP, user agent, URL, Cloud Armor decision) is separate and covered by [Privacy](../privacy.md).

`LOG_LEVEL` is `info` normally (one line per request) and can be raised to `debug` (every request, RPC, stream and game event); `debug` makes the log 5 to 10 times bigger. Change it with `gcloud run services update meurpg --region southamerica-east1 --update-env-vars LOG_LEVEL=info` (a new revision; secrets and other variables stay).

The startup lines to look for after a deploy: `starting api` (with the commit), `client ip source` (with `trusted_proxy_hops`), `rules content loaded`, `sign-in is enabled`, `images are stored in Cloud Storage`, `the images bucket is reachable`, and `image generation is on` (with the model). The warnings `DATABASE_URL is not set`, `OIDC_ISSUER is not set`, `GEMINI_API_KEY is not set` and `the images bucket could not be reached at start` each name a missing or wrong setting.

### Queries

The same commands as the [runbook](runbook.md#read-the-logs):

```bash
# The newest lines
gcloud run services logs read meurpg --region=southamerica-east1 --limit=100
# Errors in the last hour
gcloud logging read 'resource.type="cloud_run_revision" AND resource.labels.service_name="meurpg" AND severity>=ERROR' \
  --freshness=1h --limit=50 --format='value(timestamp,jsonPayload.message,jsonPayload.error)'
# One request
gcloud logging read 'resource.type="cloud_run_revision" AND jsonPayload.request_id="<id>"' --freshness=1d
# One campaign
gcloud logging read 'resource.type="cloud_run_revision" AND jsonPayload.campaign_id="<id>"' --freshness=1d
# Rate-limit hits (WARN, with the limit name and no IP)
gcloud logging read 'resource.type="cloud_run_revision" AND jsonPayload.message="rate limit hit"' --freshness=1d
# The migration job
gcloud logging read 'resource.type="cloud_run_job" AND resource.labels.job_name="meurpg-migrate"' --freshness=1d
```

## Locally with ELK

`make elk-up` starts Elasticsearch, Kibana and Filebeat (`deploy/elk/compose.yaml`); `make elk-down` stops them (the logs stay in the `esdata` volume) and `make elk-logs` shows ELK's own logs. It exists for whoever investigates bugs on a development machine; production does not use it.

| Part | Detail |
| --- | --- |
| Source | Filebeat reads the containers of the `meurpg-local` Compose project (`make up`) and the `~/.meurpg/run*/api.log` files of `LOCAL_STACK=native`, and adds `labels.stack` (`docker` or `native`) and `container.*` |
| Mapping | The ingest pipeline `meurpg-logs` (`deploy/elk/setup/pipeline.json`) maps the log contract to ECS |
| Retention | ILM policy `meurpg-logs` (`deploy/elk/setup/ilm.json`): rolls over daily or at 5 GB and deletes at 14 days. The retention is a privacy decision ([Privacy](../privacy.md#logs-in-elk)) |
| Memory | About 3.5 GB free for Docker (Elasticsearch 2 GiB with a 1 GiB heap, Kibana 1 GiB, Filebeat 512 MiB) |
| Access | Kibana at `http://localhost:5601` (user `elastic`) and Elasticsearch at `https://localhost:9200`, both on `127.0.0.1` only |
| Credentials | `deploy/elk/init-env.sh` creates `deploy/elk/.env` with random passwords the first time; the `.env` and `deploy/elk/certs/` are in `.gitignore` |
| Images | Elasticsearch, Kibana and Filebeat 9.5.5, pinned by tag and digest in `compose.yaml`. They are external services under the Elastic License 2.0; MeuRPG distributes none of them. To update, change the tag and digest on the three lines (the manifest list's digest: `docker buildx imagetools inspect <image>:<tag>`) |

The services start in order: `certs` (CA and TLS certificates, once), `es01`, `setup` (passwords, roles, ILM, pipeline, template and the `logs-meurpg-default` data stream; safe to run again), `kibana`, `kibana-setup` (the `logs-meurpg-*` data view and the saved searches) and `filebeat`.

### What the pipeline does

| Contract field | ECS field |
| --- | --- |
| `time` | `@timestamp` (`date_nanos`) |
| `severity` | `log.level`, lowercase |
| `message` | `message` |
| `service`, `version` | `service.name`, `service.version` |
| `request_id`, `user_id` | `http.request.id`, `user.id` (`keyword`) |
| `method`, `path`, `status` | `http.request.method`, `url.path`, `http.response.status_code` |
| `duration_ms` | `event.duration`, in nanoseconds |
| `error` | `error.message` |
| `logging.googleapis.com/trace` | `trace.id` (what follows `/traces/`); the whole value stays in `meurpg.gcp_trace` |
| everything else (`procedure`, `code`, `reason`, `stream`, `event`, `campaign_id`, `session_id`...) | `meurpg.<name>`; ids are `keyword`, `stream` is boolean, `messages` is a number |

A key the contract does not foresee is kept in `meurpg.*` as `keyword`; nothing rejects the document, and a wrongly typed value is stored without indexing (`ignore_malformed`). A document whose pipeline step fails enters with the tag `meurpg_pipeline_failure`.

### Reading from the terminal

`deploy/elk/logs.sh` (bash, curl and jq) prints one line per record: time, level, `req=`, message, procedure or event, `code=`, `status=`, duration and `error=`. It uses the read-only `meurpg_reader` user (`READER_PASSWORD` in `.env`).

| Command | What it shows |
| --- | --- |
| `logs.sh errors [since]` | Level `error` or a 5xx response (default 1 hour) |
| `logs.sh request <id>` | Everything one request logged, oldest first; the id is the response's `X-Request-Id` |
| `logs.sh campaign <id> [since]`, `logs.sh user <id> [since]` | One campaign or one user |
| `logs.sh search '<query>' [since]` | A Lucene `query_string`, such as `meurpg.procedure:*CreateCampaign AND NOT meurpg.code:ok` |
| `logs.sh tail [query]` | The last 10 lines, then new ones every 2 s |
| `logs.sh health`, `sample`, `simulate` | Cluster health; write four sample lines; run them through the pipeline |

`since` is a number with a unit (`30s`, `15m`, `2h`, `1d`, `1w`) or `all`. In Kibana the saved searches are "MeuRPG: erros", "RPCs lentas (> 500 ms)", "uma requisição" and "uma campanha" (the last two ask for the id in place of `COLE-O-ID-AQUI`).

### Check that it works

```bash
make elk-up                                                    # the first run downloads the images (about 2 minutes)
deploy/elk/logs.sh health                                      # cluster green (or yellow)
deploy/elk/logs.sh sample                                      # writes 4 lines to ~/.meurpg/run-sample/api.log
deploy/elk/logs.sh request 0123456789abcdef0123456789abcdef    # shows the 4 lines, in ECS, in order
make elk-down
```

If the request comes back empty, wait about 10 seconds; `make elk-logs` shows what Filebeat said.
