# syntax=docker/dockerfile:1
FROM golang:1.27.1-trixie AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/faultdeck ./cmd/faultdeck
RUN mkdir -p /out/data && chmod 0755 /out/data

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/faultdeck /faultdeck
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
WORKDIR /data
EXPOSE 7331 7332
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 CMD ["/faultdeck", "--healthcheck"]
ENTRYPOINT ["/faultdeck"]
CMD ["--listen", "0.0.0.0", "--data-dir", "/data"]
