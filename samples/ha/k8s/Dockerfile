FROM mcr.microsoft.com/oss/go/microsoft/golang:1.20.7-bullseye as builder

WORKDIR /app

COPY . .

RUN go mod download

RUN go mod tidy && go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -o /translator

FROM mcr.microsoft.com/cbl-mariner/base/core:2.0@sha256:69d5bfe4c825f7640f2e30e209ce2f66a782c549335cd0849783978f35ac86b8
RUN tdnf upgrade -y && tdnf install -y ca-certificates 
RUN tdnf install -y bind-utils tar && tdnf clean all

COPY --from=builder /translator /translator
COPY k8s/entrypoint.sh /
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/streamliner", "run", "--config", "streamliner/ha.yaml"]