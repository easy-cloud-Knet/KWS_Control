# 빌드 및 실행 단계
FROM golang:1.20

# 필수 패키지 설치 (libvirt 개발 패키지 포함)
RUN apt-get update && apt-get install -y \
    libvirt-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# 작업 디렉토리 설정
WORKDIR /app

# Go 모듈 초기화 및 의존성 설치
COPY go.mod go.sum ./
RUN go mod download

# 애플리케이션 소스 코드 복사
COPY . .

# 애플리케이션 빌드
RUN go build -o main .

# 포트 설정 (필요시)
EXPOSE 8080

# 애플리케이션 실행
CMD ["./main"]
