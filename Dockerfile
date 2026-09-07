# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static-debian12:nonroot
COPY busyhttp /busyhttp
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/busyhttp"]
