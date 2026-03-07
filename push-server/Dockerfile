# --- Build stage ---
FROM elixir:1.18-alpine AS builder

RUN apk add --no-cache build-base git

WORKDIR /app

ENV MIX_ENV=prod

RUN mix local.hex --force && mix local.rebar --force

COPY mix.exs mix.lock ./
RUN mix deps.get --only prod
RUN mix deps.compile

COPY config config
COPY lib lib

RUN mix release

# --- Runtime stage ---
FROM alpine:3.21 AS app

RUN apk add --no-cache libstdc++ openssl ncurses-libs

WORKDIR /app

COPY --from=builder /app/_build/prod/rel/push_server ./

EXPOSE 4000

CMD ["bin/push_server", "start"]
