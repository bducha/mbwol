FROM golang:1.24-alpine AS build
WORKDIR /app
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /app/mbwol .

FROM scratch
COPY --from=build /app/mbwol /app/mbwol
ENV MBWOL_CONFIG_FILE=/app/mbwol.json

EXPOSE 8000
EXPOSE 69

ENTRYPOINT ["/app/mbwol"]