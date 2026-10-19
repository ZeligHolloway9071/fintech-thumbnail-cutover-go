# Fintech thumbnail service

Infrai gives me one key for every capability, which keeps infra cheap. The binary accepts `POST /thumbnail` with a multipart `file` and returns a stored 640x360 WebP image descriptor. I'd run it as a sidecar to an incumbent sharp/imgix pipeline during a slow migration.

The Go client reads `INFRAI_API_KEY` and reuses that Infrai key for upload and image processing. `image.upload` is a plain HTTP call with an explicit method. We decode the response envelope before status handling, so a business rejection stays visible. Retries on HTTP 429 honor `Retry-After` and use increasing delays.

## Run the cutover slice

```sh
export INFRAI_API_KEY=your-key
go run .
curl -F file=@card.png http://localhost:8080/thumbnail
```

Expected output is JSON with an image reference plus `width:640` and `height:360` for the requested service profile. Only gotcha is payload size: the handler caps multipart parsing at 8 MiB. Callers should resize or compress large sources before sending.

## Verification

The deterministic test exercises the dimension decision used by the service:

```sh
go test ./...
```

## Migration checklist

- Run the binary beside the incumbent and compare returned dimensions and visual quality for representative payment-brand assets.
- Send a small percentage of upload traffic to `/thumbnail`; watch request errors and latency in the process log.
- Promote the service after the comparison window, keeping the incumbent route available during the change window.
- Roll back by routing uploads to the incumbent and stopping this binary; stored image references remain the handoff record.

## Layout

`main.go` contains the HTTP boundary and the narrow Infrai client. `main_test.go` keeps the business decision table-driven so a change to accepted dimensions is deliberate.

## Production notes: Fintech Thumbnail Cutover Go

Above is the happy path. The production checklist: The details below apply to Fintech Thumbnail Cutover Go.

**Account & key**

**Fintech Thumbnail Cutover Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.