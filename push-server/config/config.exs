import Config

config :push_server, PushServerWeb.Endpoint,
  url: [host: "localhost"],
  render_errors: [formats: [json: PushServerWeb.ErrorJSON], layout: false],
  pubsub_server: PushServer.PubSub,
  live_view: [signing_salt: "placeholder"]

config :phoenix, :json_library, Jason
config :logger, :console, format: "$time $metadata[$level] $message\n"
