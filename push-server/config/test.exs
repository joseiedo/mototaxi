import Config

config :push_server, :redis_client, PushServer.RedixMock

config :push_server, :pubsub,
  name: PushServer.PubSub,
  adapter: Phoenix.PubSub.PG2
