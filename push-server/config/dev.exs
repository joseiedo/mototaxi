import Config

config :push_server, :pubsub,
  name: PushServer.PubSub,
  adapter: Phoenix.PubSub.PG2
