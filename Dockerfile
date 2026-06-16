FROM golang:1.24 AS build

WORKDIR /app

# 의존성만 먼저 받아 레이어 캐시 활용 (소스 변경 시 재다운로드 방지)
COPY go.mod go.sum ./
RUN go mod download

# 소스 복사 후 정적 바이너리 빌드
#  - cgo 의존성이 없으므로 CGO_ENABLED=0 으로 완전 정적 빌드
#  - -trimpath, -ldflags="-s -w" 로 바이너리 경량화
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/main .

# nonroot 런타임에서 로그 파일을 쓸 수 있도록 디렉토리를 미리 생성
RUN mkdir -p /app/logs

# ---- runtime stage ----
# 정적 바이너리이므로 셸/패키지 없는 distroless static 이미지로 충분
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# 실행 바이너리
COPY --from=build /app/main .
# 런타임 설정 폴백 경로(resources/config.yaml)
COPY --from=build /app/resources ./resources
# nonroot 가 쓸 수 있는 로그 디렉토리
COPY --from=build --chown=nonroot:nonroot /app/logs ./logs

# 서버 리스닝 포트(config.yaml 의 port: 8081)와 일치
EXPOSE 8081

CMD ["./main"]
