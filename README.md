# KWS Control

- Core 노드 - 실제로 VM을 구동하는 워커 머신. Control이 HTTP로 명령을 보냄
- CMS - OVN 기반 가상 네트워크 자동화 서비스(OVS/OVN). VM 생성/삭제 시 IP / MAC / 서브넷(SDN UUID) 할당·해제
- Apache Guacamole - 브라우저에서 VM에 SSH로 접속하게 해주는 게이트웨이
- MySQL - 인스턴스 정보·서브넷 상태 영속화(메인 DB) + Guacamole 접속 설정 저장(Guacamole DB)
- Redis - VM의 최신 상태(상태값/타임스탬프) 캐시

---

## 목차

1. [아키텍처](#아키텍처)
2. [지원 환경](#지원-환경)
3. [실행](#실행)
4. [설정(Configuration)](#설정configuration)
5. [HTTP API](#http-api)
6. [데이터 저장소](#데이터-저장소)
7. [프로젝트 구조](#프로젝트-구조)
8. [테스트](#테스트)
9. [배포 (CI/CD)](#배포-cicd)

---

## 아키텍처

### VM 생성 흐름

VM 생성(`POST /vm`)요청 받았을 때.(`service/vm.go` 의 `CreateVM`)

1. 자원 요구량(CPU/MEM/DISK)을 만족하는 Core 선택 (`structure/resource_manager.go`)
2. VM 접속용 SSH 키쌍 생성 (`pkg/ssh`)
3. CMS에 서브넷/IP 할당 요청 (`Add` = 기존 서브넷 재사용 / 그 외 = 신규 서브넷)
4. Guacamole DB에 접속 설정 기록 (`pkg/guacamole`)
5. Control 인메모리 자원 테이블 갱신 -> Redis에 초기 상태 저장
6. Core에 VM 생성 요청 (`client.CoreClient`)
7. MySQL에 인스턴스 정보 영속화

Guacamole 설정·인메모리 자원 할당 단계는 `cleanupChain`(`service/cleanup.go`)에 롤백 함수를 등록하여, 이후 단계 실패 시 역순으로 정리.
단, CMS 서브넷/IP 할당은 아직 롤백이 등록되지 않아(`service/vm.go` 의 TODO), 이후 단계 실패 시 잡은 IP/서브넷은 수동 정리가 필요함.

### 계층 구조

```
api/                   HTTP 핸들러 + 요청/응답 DTO   (입력 검증, JSON 직렬화)
  └─ service/          비즈니스 로직 (오케스트레이션, 트랜잭션 흐름)
       └─ client/      외부 HTTP 클라이언트 (Core / CMS / Guacamole)
       └─ structure/   도메인 타입 · 인메모리 상태 · MySQL 영속화
       └─ pkg/         순수 유틸리티 (crypto / ssh / network / guacamole DB)
util/                  로깅 · HTTP 응답 헬퍼 (전 계층 공용)
startup/               부트스트랩 (config 읽기, DB/Redis 초기화, Core 폴링)
```

---

## 지원 환경

| 도구           | 버전                         | 용도                       |
| -------------- | ---------------------------- | -------------------------- |
| Go             | 1.23 이상 (toolchain 1.23.4) | 빌드·로컬 실행             |
| Docker         | 최신                         | 컨테이너 실행·통합 테스트  |
| Docker Compose | v2 (`docker compose`)        | 로컬 스택·테스트 스택 기동 |
| MySQL          | 8.0                          | 메인 DB + Guacamole DB     |
| Redis          | 7                            | 상태 캐시                  |

- Core 노드, CMS, Apache Guacamole가 네트워크로 도달 가능해야 실제 VM 작업이 동작.
- Docker Compose는 Redis·MySQL·Control 만 제공하며, Core/CMS/Guacamole 는 `.env` 로 외부 주소를 지정해 연결.

의존성 다운로드: `go mod download`

---

## 실행

1. `.env.example` 참고하여 환경 변수 파일 작성

2. 실행

   ```sh
   docker compose up -d --build
   ```

   - `redis` (6379), `mysql` (3306), `control_dev` 컨테이너가 실행됨.
   - MySQL 컨테이너는 첫 기동 시 `startup/init.sh` 가 자동 실행되어 DB·테이블 생성.
   - Control API는 호스트의 `8082` 포트 → 컨테이너 `8081` 로 매핑됨. (`docker-compose.yml`)

3. 동작 확인

   ```sh
   curl -i -X GET -H "Content-Type: application/json" \
     -d '{"uuid":"healthcheck"}' http://localhost:8082/vm/info
   # 서비스가 떠 있으면 404(redis 없음) 또는 200 응답이 옴.
   ```

---

## 설정

설정값은 환경 변수가 우선이고, 비어 있으면 `config.yaml` 값을 사용 (`startup/init.go`).
단, `REDIS_HOST` 는 예외로, 비어 있으면 `config.yaml` 의 `redis:` 가 아니라 하드코딩된 `localhost:6379` 로 폴백함 (`startup/redis.go`).

### 환경 변수 (`.env.example` 참고)

| 변수                                                                  | 설명                                                               |
| --------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `DB_USER` / `DB_PASSWORD` / `DB_HOST` / `DB_PORT` / `DB_NAME`         | 메인 MySQL 접속 정보 (인스턴스·서브넷 저장)                        |
| `GUAC_DB_USER` / `GUAC_DB_PASSWORD` / `GUAC_DB_HOST` / `GUAC_DB_NAME` | Guacamole MySQL 접속 정보                                          |
| `REDIS_HOST`                                                          | Redis 주소 (예: `localhost:6379`)                                  |
| `CORES`                                                               | Core 노드 주소 목록, 콤마 구분 (예: `10.0.0.5:8080,10.0.0.6:8080`) |
| `CMS_HOST`                                                            | CMS 서비스 주소 (예: `cms.internal:8080`)                          |
| `GUACAMOLE_BASE_URL`                                                  | Guacamole 베이스 URL (예: `http://host:8080/guacamole`)            |

### `resources/config.yaml`

환경 변수가 없을 때 쓰이는 기본/폴백 설정. 구조(`structure/vm.go` 의 `Config`):

```yaml
vm_internal_subnets:
  - "127.0.0.1/24"
cores:
  - "100.95.253.74:8080" # CORES 환경변수가 우선
port: 8081
db: # 메인 DB (DB_* 환경변수가 우선)
  user: "root"
  password: "password"
  host: "100.101.247.128"
  port: 3306
  name: "db"
guac_db: # Guacamole DB (GUAC_DB_* 환경변수가 우선)
  user: "root"
  password: "password"
  host: "100.101.247.128"
  port: 3306
  name: "guacamole_db"
```

> `config.yaml`(루트), `.env`, `logs/`, 빌드 산출물(`kws`)은 `.gitignore` 처리되어 있음.

---

## HTTP API

모든 라우트는 `api/handlers.go` 의 `Server()` 에 등록되며, `X-Content-Type-Options: nosniff` 헤더가 붙음.
기본 포트는 8081 (Compose dev 스택에서는 호스트 8082).

| 메서드   | 경로           | 본문 / 파라미터                             | 설명                                       |
| -------- | -------------- | ------------------------------------------- | ------------------------------------------ |
| `POST`   | `/vm`          | 아래 생성 본문                              | VM 생성                                    |
| `DELETE` | `/vm`          | `{"uuid":"..."}`                            | VM 삭제 (Core·CMS·Guacamole·DB·Redis 정리) |
| `POST`   | `/vm/start`    | `{"uuid":"..."}`                            | VM 시작(부팅)                              |
| `POST`   | `/vm/shutdown` | `{"uuid":"..."}`                            | VM 강제 종료                               |
| `GET`    | `/vm/status`   | `{"uuid":"...","type":"cpu\|memory\|disk"}` | Core에서 실시간 자원 사용량 조회           |
| `GET`    | `/vm/info`     | `{"uuid":"..."}`                            | Redis에 캐시된 VM 정보 조회                |
| `GET`    | `/vm/connect`  | `?uuid=...` (쿼리 파라미터)                 | Guacamole 인증 토큰 발급                   |
| `POST`   | `/vm/redis`    | `{"UUID":"...","status":"..."}`             | VM 상태값 갱신 (Core가 콜백)               |

### VM 생성 요청 본문 예시 (`POST /vm`)

```json
{
  "domType": "kvm",
  "domName": "my-vm",
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "os": "ubuntu-22.04",
  "HWInfo": { "cpu": 2, "memory": 4096, "disk": 20480 },
  "network": { "ips": [] },
  "users": [
    { "name": "alice", "groups": "sudo", "passWord": "secret", "ssh": [] }
  ],
  "Subnettype": "New"
}
```

- `HWInfo.memory`, `HWInfo.disk` 단위는 MiB, `cpu` 는 논리 코어 수.
- `cpu`·`memory`·`disk` 는 0이면 안 됨 (`api/create_vm.go`). `users` 는 최소 1명 필요 (`service/vm.go` 의 `CreateVM`).
- `Subnettype` 이 `"Add"` 면 기존 VM이 속한 서브넷을 재사용, 그 외 값이면 신규 서브넷을 할당함.
- 첫 번째 사용자에게는 Control이 생성한 SSH 공개키가 자동 주입됨.

> 참고: `/vm/status`, `/vm/info` 는 GET이지만 JSON 본문을 읽고, `/vm/connect` 만 쿼리 파라미터(`uuid`)를 씀.

VM 상태값(`/vm/redis`)은 `unknown`, `prepare begin`, `start begin`, `started begin`, `stopped end`,
`release end`, `migrate begin`, `restort begin` 중 하나로 정규화됨 (`api/update_redis.go`).

---

## 데이터 저장소

### MySQL - 메인 DB (`core_base`)

`startup/init.sh` / `tests/init-test-db.sql` 가 생성:

- `subnet(id, last_subnet)` - 마지막으로 할당된 서브넷 추적 (신규 서브넷 계산용)
- `inst_info(uuid, inst_ip, guac_pass, inst_mem, inst_vcpu, inst_disk)` - 인스턴스 스펙
- `inst_loc(uuid, core)` - 인스턴스가 위치한 Core 인덱스

### MySQL - Guacamole DB (`guacamole_db`)

Apache Guacamole 표준 스키마(`guacamole_entity`, `guacamole_user`, `guacamole_connection`,
`guacamole_connection_parameter`, `guacamole_connection_permission`). 전체 DDL은 [database.md](database.md) 참고.
Control은 VM 생성 시 이 테이블에 직접 사용자·SSH 커넥션을 기록함 (`pkg/guacamole/config.go`).

### Redis

키 = VM UUID, 값 = `{uuid, cpu, memory, disk, ip, status, time}` JSON (`service/redis.go`).

---

## 프로젝트 구조

```
KWS_Control/
├── main.go                  # 진입점: Redis·Core 데이터 초기화 후 HTTP 서버 기동
├── Makefile                 # build / run / clean 타깃 (kws 바이너리)
├── Dockerfile               # golang:1.23 기반 빌드 이미지
├── docker-compose.yml       # 로컬 스택: redis + mysql + control_dev
├── .env.example             # 환경 변수 템플릿
├── database.md              # Guacamole DB 스키마(DDL) 참고 문서
├── go.mod / go.sum          # Go 모듈 정의
│
├── api/                     # ── HTTP 계층 (핸들러 + 요청/응답 DTO) ──
│   ├── handlers.go          #   라우터: Server(), 모든 라우트 등록
│   ├── middleware.go        #   공통 보안 헤더 미들웨어
│   ├── create_vm.go         #   POST   /vm           VM 생성
│   ├── delete_vm.go         #   DELETE /vm           VM 삭제
│   ├── start_vm.go          #   POST   /vm/start     VM 시작
│   ├── shutdown_vm.go       #   POST   /vm/shutdown  VM 강제 종료
│   ├── get_vm_status.go     #   GET    /vm/status    실시간 자원 조회
│   ├── get_vm_info.go       #   GET    /vm/info      Redis 캐시 조회
│   ├── connect_vm.go        #   GET    /vm/connect   Guacamole 토큰 발급
│   └── update_redis.go      #   POST   /vm/redis     상태 갱신 + 상태 상수
│
├── service/                 # ── 비즈니스 로직 (오케스트레이션) ──
│   ├── vm.go                #   CreateVM/DeleteVM/StartVM/ShutdownVM/Get*Info
│   ├── network.go           #   CMS 서브넷 할당·삭제(Add/New), VM IP 조회
│   ├── guacamole.go         #   Guacamole 인증 토큰 발급 로직
│   ├── redis.go             #   VM 정보 Redis 저장/조회/갱신/삭제 + 상태 상수
│   ├── dto.go               #   서비스 계층 입출력 DTO
│   ├── cleanup.go           #   cleanupChain (단계별 롤백 체인)
│   └── core_allocation.go   #   (미사용) 코어 라운드로빈 할당 자리표시
│
├── client/                  # ── 외부 시스템 HTTP 클라이언트 ──
│   ├── vm.go                #   CoreClient: Core 노드에 VM 명령/상태 조회
│   ├── cms.go               #   CmsClient: 인스턴스(IP/MAC/서브넷) 할당·삭제
│   ├── guacamole.go         #   GuacamoleClient: REST 인증 토큰 획득
│   └── model/
│       ├── vm.go            #     Core/VM 요청·응답 계약 + 상태 상수
│       └── common.go        #     제네릭 CoreResponse[T] + 에러 타입
│
├── structure/               # ── 도메인 타입 · 상태 · 영속화 ──
│   ├── vm.go                #   Config, Core, CoreInfo, VMInfo, UUID 등 핵심 타입
│   ├── control_infra.go     #   ControlContext + UUID→Core 조회
│   ├── resource_manager.go  #   인메모리 자원 관리: 코어 선택, 할당/회수, 락
│   ├── repository.go        #   VMRepository 인터페이스
│   ├── mysql_vm_repository.go#  VMRepository의 MySQL 구현체
│   └── errors.go            #   도메인 에러 생성자
│
├── pkg/                     # ── 재사용 가능한 순수 유틸리티 ──
│   ├── crypto/password.go   #   Salt/SHA256 해시·랜덤 비밀번호 (Guacamole 호환)
│   ├── guacamole/config.go  #   Guacamole DB에 사용자·SSH 커넥션 생성/정리
│   ├── network/network.go   #   서브넷 계산 (다음 서브넷, IP→서브넷)
│   └── ssh/keygen.go        #   RSA SSH 키쌍 생성
│
├── startup/                 # ── 부트스트랩 ──
│   ├── init.go              #   InitializeCoreData: config·DB 연결·Core 폴링·인스턴스 로드
│   ├── redis.go             #   InitializeRedis: 연결·헬스 체크
│   ├── core_ip_config.go    #   readConfig: config.yaml 파싱(+ resources/ 폴백)
│   └── init.sh              #   MySQL 컨테이너 초기 DB/테이블 생성 스크립트
│
├── util/                    # ── 전 계층 공용 헬퍼 ──
│   ├── logger.go            #   logrus 기반 커스텀 로거(파일+stdout, 호출 위치 표기)
│   ├── response.go          #   RespondJSON / RespondError
│   └── util.go              #   (빈 자리표시 파일)
│
├── tests/                   # ── 블랙박스 통합 테스트 ──
│   ├── blackbox_io_test.sh  #   전체 라이프사이클 검증 러너
│   ├── docker-compose.test.yml#  테스트 스택(mysql/redis/mock-core/control)
│   ├── init-test-db.sql     #   테스트 DB 스키마 시드
│   └── mock_core.py         #   Core/CMS/Guacamole 목(mock) 서버
│
├── resources/
│   └── config.yaml          #   기본/폴백 설정값
│
├── .github/
│   ├── workflows/dev.yaml   #   staging 푸시 → control_dev 이미지 빌드·배포
│   ├── workflows/deploy.yaml#   production 푸시 → control_deploy 이미지 빌드·배포
│   ├── ISSUE_TEMPLATE/      #   이슈 템플릿(bug/feature/task)
│   └── PULL_REQUEST_TEMPLATE.md
│
└── .vscode/launch.json      #   VS Code Go 디버그 설정
```

---

## 테스트

외부 의존성을 모두 컨테이너(목 포함)로 띄우는 블랙박스 통합 테스트 제공.
Docker / Docker Compose v2 만 있으면 실행 가능.

```sh
./tests/blackbox_io_test.sh
```

- `tests/docker-compose.test.yml` 로 MySQL · Redis · 목 Core(`mock_core.py`) · Control 을 기동함.
- HTTP 라우팅(405), 본문 검증(400), 필드 검증, 라이프사이클 시나리오를 차례로 검증.
- 종료 시 테스트 스택과 볼륨을 자동 정리함(`trap cleanup EXIT`).
- Control 테스트 서비스는 호스트 `18081` 포트로 노출.

---

## 배포 (CI/CD)

`self-hosted` 러너(로컬 Ubuntu)에서 Docker 이미지를 빌드·배포.

| 브랜치       | 워크플로                        | 컨테이너         | 포트(호스트→컨테이너) |
| ------------ | ------------------------------- | ---------------- | --------------------- |
| `staging`    | `.github/workflows/dev.yaml`    | `CONTROL_DEV`    | `8082 → 8081`         |
| `production` | `.github/workflows/deploy.yaml` | `CONTROL_DEPLOY` | `8081 → 8081`         |

- PR의 기본 대상 브랜치는 `staging`.
- 배포에 필요한 값(`CORES`, `DB_*`, `GUAC_DB_*`, `REDIS_HOST`, `CMS_HOST`, `GUAC_BASE_URL` 등)은 GitHub Actions Secrets 로 주입.
